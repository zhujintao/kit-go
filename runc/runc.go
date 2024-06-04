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
	process *libcontainer.Process
	*libcontainer.Container
}

type NewGroup []createOpts

// archive image path
func Container(image string, opts ...createOpts) *container {

	s := &specconv.CreateOpts{
		UseSystemdCgroup: false,
		NoPivotRoot:      false,
		NoNewKeyring:     false,
		Spec:             defaultSpec(),
		RootlessEUID:     os.Geteuid() != 0,
		RootlessCgroups:  false,
	}
	s.CgroupName = s.Spec.Hostname

	for _, o := range opts {
		err := o(s)
		if err != nil {
			log.Debug("opts", err)
		}
	}

	id := s.Spec.Hostname
	c, err := libcontainer.Load(repo, id)
	if err == nil {
		parserImage(image, true)(s)
		p, err := newProcess(s.Spec.Process)
		if err != nil {
			panic(err)
		}
		log.Info("load container", "id", id)
		return &container{Container: c, process: p}
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
		log.Error(err.Error())
		panic(err)
	}
	parserImage(image, false)(s)
	p, err := newProcess(s.Spec.Process)
	if err != nil {
		panic(err)
	}
	log.Info("new container", "id", id)
	return &container{Container: c, process: p}
}

// 1.create
// 2.run
func (c *container) Start() error {

	err := c.Container.Start(c.process)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	return nil
}

// start -> run, dea
func (c *container) Run() {

	status, err := c.Status()
	if err != nil {
		panic(err)
	}

	switch status {
	case libcontainer.Created:
		err := c.Container.Exec()
		if err != nil {
			log.Error(err.Error(), "id", c.ID())
		}
		return
	case libcontainer.Stopped:
		c.runContainer()

	case libcontainer.Running:
		log.Info("cannot start an already running container")
		return
	default:
		log.Info("cannot start a container", "state", status)
		return
	}

}

func (c *container) Exec(cmd ...string) {
	status, err := c.Status()
	if err != nil {
		log.Error(err.Error())
		return
	}
	if status != libcontainer.Running {
		log.Error("cannot exec in a stopped container")
		return
	}

	c.process.Init = false
	c.Container.Run(c.process)

}

func (c *container) runContainer() error {
	const signalBufferSize = 2048

	if c.process == nil {
		log.Error("Process not set")
		return fmt.Errorf("Process not set")
	}

	signals := make(chan os.Signal, signalBufferSize)
	signal.Notify(signals)

	err := c.Container.Run(c.process)
	if err != nil {
		c.Destroy()
		log.Error(err.Error())
		return err
	}

	pid1, err := c.process.Pid()
	if err != nil {
		log.Error(err.Error())
		return err
	}
	log.Info(fmt.Sprintf("pid: %d", pid1), "id", c.ID())

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
					return nil
				}
			}
		case unix.SIGURG:
		default:
			us := s.(unix.Signal)
			if err := unix.Kill(pid1, us); err != nil {
				log.Error(err.Error())
			}
		}

	}

	c.Destroy()
	return nil

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
