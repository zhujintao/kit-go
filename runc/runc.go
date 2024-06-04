package runc

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runc/libcontainer/utils"
	"golang.org/x/sys/unix"
)

var repo string = "/var/lib/libcontainer"

type container struct {
	id      string
	process *libcontainer.Process
	*libcontainer.Container
}

type NewGroup []createOpts

func Container(image string, opts ...createOpts) *container {

	s := &specconv.CreateOpts{
		UseSystemdCgroup: false,
		NoPivotRoot:      false,
		NoNewKeyring:     false,
		Spec:             defaultSpec(),
		RootlessEUID:     os.Geteuid() != 0,
		RootlessCgroups:  false,
	}

	for _, o := range opts {
		err := o(s)
		if err != nil {
			fmt.Println("opts", err)
		}
	}

	fmt.Println(truncateID(generateID()))

	id := s.CgroupName
	if s.Spec.Hostname == "" {
		s.Spec.Hostname = id
	}

	c, err := libcontainer.Load(repo, id)
	if err == nil {
		parserImage(image, true)(s)
		p, err := newProcess(s.Spec.Process)
		if err != nil {
			panic(err)
		}
		log.Info("load container", "id", id)
		return &container{Container: c, process: p, id: id}
	}

	_, err = os.Stat(s.Spec.Root.Path)
	if os.IsNotExist(err) {
		log.Error("rootfs dir not exist, use OptWithRootPath")
		os.Exit(1)

	}

	config, err := specconv.CreateLibcontainerConfig(s)
	if err != nil {
		panic(err)
	}

	c, err = libcontainer.Create(repo, id, config)
	if err != nil {
		log.Error("libcontainer.Create", err)
		panic(err)
	}
	parserImage(image, false)(s)
	p, err := newProcess(s.Spec.Process)
	if err != nil {
		panic(err)
	}
	log.Info("new container", "id", id)
	return &container{Container: c, process: p, id: id}
}

func (c *container) Run() {

	status, err := c.Status()
	if err != nil {
		panic(err)
	}
	log.Info(status.String(), "id", c.ID())

	switch status {
	case libcontainer.Created:

		c.Exec()
		return
	case libcontainer.Stopped:
		c.runContainer()
		return
	case libcontainer.Running:
		log.Info("cannot start an already running container")
		return
	default:
		log.Info("cannot start a container", "state", status)
		return
	}

}

func (c *container) runContainer() {
	const signalBufferSize = 2048

	if c.process == nil {
		log.Error("Process not set")
		return
	}

	signals := make(chan os.Signal, signalBufferSize)
	signal.Notify(signals)

	err := c.Container.Run(c.process)
	fmt.Println("Container.Run", err)
	if err != nil {
		c.Destroy()
		return
	}

	pid1, err := c.process.Pid()
	fmt.Println("pid", pid1)
	if err != nil {
		fmt.Println("process.Pid", err)
		return
	}

	for s := range signals {

		switch s {
		case unix.SIGWINCH:
		case unix.SIGCHLD:
			exits, err := reap()
			if err != nil {
				fmt.Println("reap()", err)
			}
			for _, e := range exits {
				if e.pid == pid1 {
					c.process.Wait()
					return
				}
			}
		case unix.SIGURG:
		default:
			us := s.(unix.Signal)
			if err := unix.Kill(pid1, us); err != nil {
				fmt.Println("kill", err)
			}
		}

	}

	c.Destroy()

}

func (c *container) Restore() {
	err := c.Container.Restore(c.process, nil)
	fmt.Println(err)
}

type exit struct {
	pid    int
	status int
}

func reap() (exits []exit, err error) {
	var (
		ws  unix.WaitStatus
		rus unix.Rusage
	)
	for {
		pid, err := unix.Wait4(-1, &ws, unix.WNOHANG, &rus)
		if err != nil {
			if err == unix.ECHILD {
				return exits, nil
			}
			return nil, err
		}
		if pid <= 0 {
			return exits, nil
		}
		exits = append(exits, exit{
			pid:    pid,
			status: utils.ExitStatus(ws),
		})
	}
}
