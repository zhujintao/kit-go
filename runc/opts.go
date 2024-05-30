package runc

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

type createOpts func(c *specconv.CreateOpts) error

func OptWithSetId(id string) createOpts {
	return func(c *specconv.CreateOpts) error {

		c.CgroupName = id
		return nil
	}
}

// use oci opts  "github.com/containerd/containerd/oci"
func OptWithOciSpec(opts ...oci.SpecOpts) createOpts {
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

func skipFunc(s string, substrs ...string) bool {

	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false

}
