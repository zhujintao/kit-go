package runc

import (
	"fmt"
	"log/slog"
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

func Container(id string, opts ...createOpts) *container {

	s := &specconv.CreateOpts{
		CgroupName: id,
	}
	s.Spec = defaultSpec(id)

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

func (c container) Run() {
	const signalBufferSize = 2048
	signals := make(chan os.Signal, signalBufferSize)
	signal.Notify(signals)
	err := c.Container.Run(c.process)
	if err != nil {
		c.Destroy()
	}

	pid1, err := c.process.Pid()
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

func (c container) Restore() {
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
