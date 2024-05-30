package runc

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

var repo string = "/var/lib/libcontainer"

type container struct {
	process *libcontainer.Process
	*libcontainer.Container
}

func Container(id string, opts ...createOpts) *container {

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

	c, err := libcontainer.Load(repo, id)
	if err == nil {
		return &container{Container: c}
	}

	_, err = os.Stat(s.Spec.Root.Path)
	if os.IsNotExist(err) {
		slog.Error("rootfs dir not exist, use OptWithRootPath")
		os.Exit(1)

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
	return &container{Container: c, process: p}
}

func (c container) AsRun() {

	err := c.Run(c.process)
	if err != nil {
		c.Destroy()
	}
	c.process.Wait()
	c.Destroy()
}
