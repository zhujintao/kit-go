package runc

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

type NewContainerOpts func(c *specconv.CreateOpts) error

// use oci opts  "github.com/containerd/containerd/oci"
func WithOciSpec(opts ...oci.SpecOpts) NewContainerOpts {
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
