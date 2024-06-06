package runc

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/containerd/containerd/cio"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/sirupsen/logrus"
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
	log.Info("new container", "id", c.ID())
	return &container{Container: c, process: p}
}

// create container
func (c *container) Create() error {

	err := c.Container.Start(c.process)
	if err != nil {
		log.Error(err.Error(), "id", c.ID())
		return err
	}
	return nil

}

// run a container, automatically remove the container when it exits
func (c *container) RunOne(detach bool) error {
	err := c.run(detach)
	if err != nil {
		return err
	}

	return c.Destroy()

}

// run a container
func (c *container) Run(detach bool) error {
	return c.run(detach)
}

func (c *container) run(detach bool) error {

	status, err := c.Status()
	if err != nil {
		log.Error(err.Error(), "id", c.ID())
		return err
	}

	switch status {
	case libcontainer.Created:
		err := c.Container.Exec()
		if err != nil {
			log.Error(err.Error(), "id", c.ID())
			return err
		}
		return nil
	case libcontainer.Stopped:
		handler := newSignalHandler()
		err := c.Container.Run(c.process)
		if err != nil {
			c.Destroy()
			log.Error(err.Error(), "id", c.ID())
			return err
		}

		status, err := handler.forward(c.process, detach)
		if err != nil {
			c.process.Signal(unix.SIGKILL)
			c.process.Wait()
		}

		if err == nil {
			os.Exit(status)
		}

	case libcontainer.Running:
		log.Info("cannot start an already running container", "id", c.ID())
		return nil
	default:
		log.Info("cannot start a container", "state", status, "id", c.ID())
		return nil
	}

	return nil

}

// execute additional processes in an existing container
func (c *container) Exec(cmd ...string) {
	status, err := c.Status()
	if err != nil {
		log.Error(err.Error())
		return
	}
	if status != libcontainer.Running {
		log.Error("cannot exec in a stopped container", "id", c.ID())
		return
	}

	c.process.Init = false
	c.process.Args = cmd
	err = c.Container.Run(c.process)
	if err != nil {
		log.Error(err.Error(), "id", c.ID())
	}
}

func (c *container) Attach() error {
	state, err := c.State()
	if err != nil {
		log.Error(err.Error(), "id", c.ID())
		return err
	}

	fifos := cio.NewFIFOSet(cio.Config{
		Stdin:  fmt.Sprintf("/proc/%d/fd/0", state.InitProcessPid),
		Stdout: fmt.Sprintf("/proc/%d/fd/1", state.InitProcessPid),
		Stderr: fmt.Sprintf("/proc/%d/fd/2", state.InitProcessPid),
	}, nil)

	ioAttach := cio.NewAttach(cio.WithStdio)
	io, err := ioAttach(fifos)
	if err != nil {
		log.Error(err.Error(), "id", c.ID())
		return err
	}

	io.Wait()

	defer func() {
		io.Cancel()
		io.Wait()
		io.Close()
	}()

	return nil

}

type signalHandler struct {
	signals chan os.Signal
}

func newSignalHandler() *signalHandler {
	const signalBufferSize = 2048
	signals := make(chan os.Signal, signalBufferSize)
	signal.Notify(signals)

	return &signalHandler{signals: signals}

}
func (h *signalHandler) reap() (exits []exit, err error) {
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
func (h *signalHandler) forward(process *libcontainer.Process, detach bool) (int, error) {

	if detach {
		return 0, nil
	}
	pid1, err := process.Pid()
	if err != nil {
		return -1, err
	}
	for s := range h.signals {
		switch s {
		case unix.SIGWINCH:
			// Ignore errors resizing, as above.

		case unix.SIGCHLD:
			exits, err := h.reap()
			if err != nil {
				logrus.Error(err)
			}
			for _, e := range exits {

				log.Debug("process exited", "pid", e.pid, "status", e.status)

				if e.pid == pid1 {
					// call Wait() on the process even though we already have the exit
					// status because we must ensure that any of the go specific process
					// fun such as flushing pipes are complete before we return.
					_, _ = process.Wait()
					return e.status, nil
				}
			}
		case unix.SIGURG:
			// SIGURG is used by go runtime for async preemptive
			// scheduling, so runc receives it from time to time,
			// and it should not be forwarded to the container.
			// Do nothing.
		default:
			us := s.(unix.Signal)
			log.Debug(fmt.Sprintf("forwarding signal %d (%s) to %d", int(us), unix.SignalName(us), pid1))

			if err := unix.Kill(pid1, us); err != nil {
				log.Error(err.Error())
			}
		}
	}

	return -1, nil

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
