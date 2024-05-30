package runc

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

var repo string = "/var/lib/libcontainer"

type container struct {
	process   *libcontainer.Process
	container *libcontainer.Container
}

func (c container) Run() {

	c.container.Run(c.process)
	c.process.Wait()
}

func Container(id string, opts ...createOpts) *container {

	for _, o := range opts {
		err := o(&specconv.CreateOpts{})
		if err != nil {
			fmt.Println("opts", err)
		}
	}

	c, err := libcontainer.Load(repo, id)
	if err == nil {

		return &container{container: c}
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

	c, err = libcontainer.Create(repo, id, config)
	if err != nil {

		panic(err)
	}

	p, err := newProcess(*s.Spec.Process)
	if err != nil {
		panic(err)
	}
	return &container{container: c, process: p}
}

func Container2(id string, opts ...createOpts) *libcontainer.Container {

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
