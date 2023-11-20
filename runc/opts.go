package runc

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/zhujintao/kit-go/image"
)

type NewContainerOpts func(c *specconv.CreateOpts) error

func WithOciSpec(opts ...oci.SpecOpts) NewContainerOpts {

	return func(c *specconv.CreateOpts) error {

		for _, o := range opts {
			name := runtime.FuncForPC(reflect.ValueOf(o).Pointer()).Name()
			if skipFunc(name, "WithDefaultSpec") {
				continue
			}

			if err := o(nil, nil, nil, c.Spec); err != nil {
				return err
			}
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

func defaultSpec() *specs.Spec {

	tmpl, err := generate.New(runtime.GOOS)
	if err != nil {
		return nil
	}
	return tmpl.Config
}

func WithRootlessCgroups(b bool) NewContainerOpts {
	return func(c *specconv.CreateOpts) error {
		c.RootlessCgroups = b
		return nil
	}
}
func WithRootlessEUID(b bool) NewContainerOpts {
	return func(c *specconv.CreateOpts) error {
		c.RootlessEUID = b
		return nil
	}
}
func WithNoNewKeyring(b bool) NewContainerOpts {
	return func(c *specconv.CreateOpts) error {
		c.NoNewKeyring = b
		return nil
	}
}
func WithNoPivotRoot(b bool) NewContainerOpts {
	return func(c *specconv.CreateOpts) error {
		c.NoPivotRoot = b
		return nil
	}
}
func WithUseSystemdCgroup(b bool) NewContainerOpts {
	return func(c *specconv.CreateOpts) error {
		c.UseSystemdCgroup = b
		return nil
	}
}
func WithImage(ref string) NewContainerOpts {
	return func(c *specconv.CreateOpts) error {
		err := image.Fetch(ref, true)
		if err != nil {
			return err
		}
		rootfs := filepath.Join(taskDir, c.CgroupName, "rootfs")
		err = os.MkdirAll(rootfs, 0700)
		if err != nil {
			return nil
		}
		err = image.Mount(ref, c.CgroupName, rootfs)
		if err != nil {
			return nil
		}
		err = os.Chdir(filepath.Join(taskDir, c.CgroupName))
		if err != nil {
			return err
		}
		o := image.WithImageConfig(ref)
		err = o(nil, nil, nil, c.Spec)
		if err != nil {
			return err
		}
		c.Spec.Annotations = map[string]string{"image": ref, "command": strings.Join(c.Spec.Process.Args, " ")}

		return nil
	}
}
