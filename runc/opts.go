package runc

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/containerd/containerd/oci"
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
