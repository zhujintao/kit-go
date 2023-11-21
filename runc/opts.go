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

func defaultSpec() *specs.Spec {

	tmpl, err := generate.New(runtime.GOOS)
	if err != nil {
		return nil
	}
	tmpl.Config.Process.Args = []string{""}
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
		c.Spec.Annotations["image"] = ref
		c.Spec.Annotations["command"] = strings.Join(c.Spec.Process.Args, " ")

		return nil
	}
}

type volume struct {
	ms []specs.Mount
}

func (v *volume) SetVolume(source, destination string) *volume {
	v.ms = append(v.ms, specs.Mount{
		Type:        "bind",
		Options:     []string{"bind"},
		Source:      source,
		Destination: destination,
	})
	return v
}
func (v *volume) Do() []specs.Mount {
	return v.ms
}

func WithVolume(source, destination string, options ...string) NewContainerOpts {
	if len(options) != 1 {
		options = []string{"bind"}
	}
	return func(c *specconv.CreateOpts) error {

		m := specs.Mount{
			Type:        "bind",
			Options:     options,
			Source:      source,
			Destination: destination,
		}

		c.Spec.Mounts = append(c.Spec.Mounts, m)
		return nil
	}

}

func WithEnv(env string) NewContainerOpts {
	return func(c *specconv.CreateOpts) error {

		c.Spec.Process.Env = append(c.Spec.Process.Env, strings.Fields(env)...)
		return nil

	}

}

// WithProcessArgs replaces the cmd and args
//
// cmd is "" original cmd unchanged
func WithProcessArgs(cmd string, args ...string) NewContainerOpts {
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
