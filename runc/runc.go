package runc

import (
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

type runc struct {
	*libcontainer.Container
}

func SetId(id string) NewContainerOpts {
	return func(c *specconv.CreateOpts) error {

		c.CgroupName = id
		return nil
	}
}

func Container(id string, opts ...NewContainerOpts) *libcontainer.Container {

	c, err := libcontainer.Load("", id)
	if err == nil {
		return c
	}

	c, err = libcontainer.Create("", "", &configs.Config{})
	if err != nil {

		panic(err)
	}
	return c

}
