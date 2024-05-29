package runc

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

func OptWithSetId(id string) createOpts {
	return func(c *specconv.CreateOpts) error {

		c.CgroupName = id
		return nil
	}
}

func Container(id string, opts ...createOpts) *libcontainer.Container {

	c, err := libcontainer.Load("", id)
	if err == nil {
		return c
	}

	s := &specconv.CreateOpts{
		CgroupName: id,
	}
	s.Spec = defaultSpec(id)
	s.Spec.Linux.Seccomp = nil

	for _, o := range opts {
		err := o(s)
		if err != nil {
			fmt.Println("opts", err)
		}
	}

	config, err := specconv.CreateLibcontainerConfig(s)
	if err != nil {
		panic(err)
	}

	c, err = libcontainer.Create("", id, config)
	if err != nil {

		panic(err)
	}
	return c

}
