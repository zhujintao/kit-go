package runc

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"runtime"
	"strings"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

type createOpts func(c *specconv.CreateOpts) error

// use oci opts  "github.com/containerd/containerd/oci"
func SetWithOciSpec(opts ...oci.SpecOpts) createOpts {
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
func SetRepoPath(root string) createOpts {
	return func(c *specconv.CreateOpts) error {
		repo = root
		return nil
	}

}

// path is the absolute path to the container's root filesystem.
func SetRootPath(path string) createOpts {
	return func(c *specconv.CreateOpts) error {
		c.Spec.Root.Path = path
		return nil
	}

}

func SetEnv(env string) createOpts {
	return func(c *specconv.CreateOpts) error {

		c.Spec.Process.Env = append(c.Spec.Process.Env, strings.Fields(env)...)
		return nil

	}

}

// setArgs replaces the cmd and args
//
// cmd is "" original cmd unchanged
func SetArgs(cmd string, args ...string) createOpts {
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

func SetId(id string) createOpts {
	return func(c *specconv.CreateOpts) error {
		c.CgroupName = id
		return nil
	}
}

// archive image path
func setImage(image string, onlyConfig bool) createOpts {
	return func(c *specconv.CreateOpts) error {

		reader, err := os.Open(image)
		if err != nil {
			return err
		}

		var (
			tr        = tar.NewReader(reader)
			ociLayout ocispec.ImageLayout
			mfsts     []struct {
				Config   string
				RepoTags []string
				Layers   []string
			}
			imageConfigBytes []byte
			ociimage         ocispec.Image

			buf bytes.Buffer
		)

		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil
			}

			if onlyConfig {

				imageConfigBytes, _ = io.ReadAll(tr)
				json.Unmarshal(imageConfigBytes, &ociimage)
				if reflect.DeepEqual(ociimage.Config, ocispec.ImageConfig{}) {
					continue
				}
				if reflect.DeepEqual(ociimage, ocispec.Image{}) {
					continue
				}

				setImageConfig(c.Spec, ociimage.Config)

				continue
			}

			if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
				if hdr.Typeflag != tar.TypeDir {
					fmt.Println("file", hdr.Name, "file type ignored")
				}
				continue
			}
			hdrName := path.Clean(hdr.Name)
			if hdrName == ocispec.ImageLayoutFile {
				if err = onUntarJSON(tr, &ociLayout); err != nil {
					return fmt.Errorf("untar oci layout %q: %w", hdr.Name, err)
				}

			} else if hdrName == "manifest.json" {
				if err = onUntarJSON(tr, &mfsts); err != nil {
					return fmt.Errorf("untar manifest %q: %w", hdr.Name, err)
				}

			} else {

				tee := io.TeeReader(tr, &buf)
				s, _ := compression.DecompressStream(tee)
				if _, err := archive.Apply(context.Background(), "rootfs", s); err == nil {
					fmt.Println("apply", hdrName)
					continue
				}
				imageConfigBytes, _ = io.ReadAll(&buf)
				json.Unmarshal(imageConfigBytes, &ociimage)
				if reflect.DeepEqual(ociimage.Config, ocispec.ImageConfig{}) {
					continue
				}
				if reflect.DeepEqual(ociimage, ocispec.Image{}) {
					continue
				}

				setImageConfig(c.Spec, ociimage.Config)

			}

		}

		return nil
	}
}

func readImageConfig(s *oci.Spec, image string) error {

	reader, err := os.Open(image)
	if err != nil {
		return err
	}

	tr := tar.NewReader(reader)

	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}

		var (
			imageConfigBytes []byte
			ociimage         ocispec.Image
		)

		imageConfigBytes, _ = io.ReadAll(tr)
		json.Unmarshal(imageConfigBytes, &ociimage)
		if reflect.DeepEqual(ociimage.Config, ocispec.ImageConfig{}) {
			continue
		}

		fmt.Println(ociimage.Config)
		setImageConfig(s, ociimage.Config)

	}

	return nil

}

func onUntarJSON(r io.Reader, j interface{}) error {

	const (
		kib       = 1024
		mib       = 1024 * kib
		jsonLimit = 20 * mib
	)

	return json.NewDecoder(io.LimitReader(r, jsonLimit)).Decode(j)
}

func onUntarBlob(ctx context.Context) error {

	return nil
}

func setImageConfig(s *oci.Spec, config ocispec.ImageConfig) {
	s.Process.Env = config.Env
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
}
