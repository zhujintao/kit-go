package runc

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type createOpts func(c *specconv.CreateOpts) error

// use oci opts  "github.com/containerd/containerd/oci"
func WithContainerOciSpec(opts ...oci.SpecOpts) createOpts {
	return func(c *specconv.CreateOpts) error {

		for _, o := range opts {
			name := runtime.FuncForPC(reflect.ValueOf(o).Pointer()).Name()

			// skip oci.WithDefaultSpec
			if skipFunc(name, "WithDefaultSpec") {
				continue
			}

			if err := o(nil, nil, nil, c.Spec); err != nil {

				return err
			}

			c.Spec.Annotations["command"] = strings.Join(c.Spec.Process.Args, " ")
		}
		return nil
	}

}

func skipFunc(s string, substrs ...string) bool {

	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false

}

// default /var/lib/libcontainer
func SetConfigRepoPath(root string) createOpts {
	return func(c *specconv.CreateOpts) error {
		repo = root
		return nil
	}

}

// path is the absolute path to the container's root filesystem.
func SetConfigRootPath(path string) createOpts {
	return func(c *specconv.CreateOpts) error {
		c.Spec.Root.Path = path
		return nil
	}

}

func WithContainerEnv(env string) createOpts {
	return func(c *specconv.CreateOpts) error {
		oci.WithEnv(strings.Fields(env))(nil, nil, nil, c.Spec)
		return nil

	}

}

// setArgs replaces the cmd and args
//
// cmd is "" original cmd unchanged
func WithContainerArgs(cmd string, args ...string) createOpts {
	return func(c *specconv.CreateOpts) error {

		if cmd != "" {
			c.Spec.Process.Args = []string{cmd}
		}

		if len(args) != 0 {
			c.Spec.Process.Args = append(c.Spec.Process.Args, args...)
		}
		return nil
	}
}

func WithContainerId(id string) createOpts {
	return func(c *specconv.CreateOpts) error {

		c.CgroupName = id
		c.Spec.Hostname = id
		return nil
	}
}

// archive image path
func parserImage(id, image string, onlyConfig bool) createOpts {

	return func(c *specconv.CreateOpts) error {
		var (
			ociimage ocispec.Image
			mfsts    []struct {
				Config   string
				RepoTags []string
				Layers   []string
			}
		)

		manifestFn := func(tr *tar.Reader, hdr *tar.Header) {
			if path.Clean(hdr.Name) == "manifest.json" {
				onUntarJSON(tr, &mfsts)
			}
		}
		tarFor(image, manifestFn)

		configfn := func(tr *tar.Reader, hdr *tar.Header) {
			for _, mfst := range mfsts {
				if path.Clean(hdr.Name) == mfst.Config {
					onUntarJSON(tr, &ociimage)
					log.Debug("config - "+mfst.Config, "id", id)
				}
			}
		}

		tarFor(image, configfn)
		setImageConfig(c.Spec, ociimage.Config)
		if onlyConfig {
			return nil
		}
		vol := filepath.Join(volrepo, id)
		for _, mfst := range mfsts {
			for _, layer := range mfst.Layers {
				tarFor(image, func(tr *tar.Reader, hdr *tar.Header) {
					if path.Clean(hdr.Name) == layer {
						s, _ := compression.DecompressStream(tr)
						archive.Apply(context.Background(), vol, s)
						log.Debug("apply - "+layer, "id", id)
					}
				})
			}
		}

		return nil
	}

}

func onUntarJSON(r io.Reader, j interface{}) error {

	const (
		kib       = 1024
		mib       = 1024 * kib
		jsonLimit = 20 * mib
	)

	return json.NewDecoder(io.LimitReader(r, jsonLimit)).Decode(j)
}

func setImageConfig(s *oci.Spec, config ocispec.ImageConfig) {
	defaults := config.Env

	oci.WithDefaultPathEnv(context.TODO(), nil, nil, s)

	s.Process.Env = append(s.Process.Env, defaults...)
	cmd := config.Cmd
	if s.Process.Args[0] == "" {
		cmd = append(cmd, s.Process.Args[1:]...)
		s.Process.Args = append(config.Entrypoint, cmd...)
	}
	cwd := config.WorkingDir
	if cwd == "" {
		cwd = "/"
	}
	s.Process.Cwd = cwd
	s.Annotations["stop-signal"] = config.StopSignal

	s.Process.User = specs.User{
		Username: config.User}

}

//svcs[i].ContainerName = fmt.Sprintf("%[1]s%[4]s%[2]s%[4]srun%[4]s%[3]s", c.project.Name, svcs[i].Name, idgen.TruncateID(idgen.GenerateID()), serviceparser.Separator)

const (
	IDLength      = 64
	ShortIDLength = 12
)

func generateID() string {

	return _truncateID(_generateID())
}
func _generateID() string {
	bytesLength := IDLength / 2
	b := make([]byte, bytesLength)
	n, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	if n != bytesLength {
		panic(fmt.Errorf("expected %d bytes, got %d bytes", bytesLength, n))
	}
	return hex.EncodeToString(b)

}

func _truncateID(id string) string {
	if len(id) < ShortIDLength {
		return id
	}
	return id[:ShortIDLength]
}

func tarFor(image string, fn func(tr *tar.Reader, hdr *tar.Header)) {

	reader, err := os.Open(image)
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer reader.Close()
	tr := tar.NewReader(reader)

	for {

		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return
		}

		fn(tr, hdr)

	}

}
