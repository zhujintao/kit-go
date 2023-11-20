package runc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"text/tabwriter"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/moby/sys/user"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/zhujintao/kit-go/image"
	"golang.org/x/sys/unix"
)

var repo string
var taskDir string

func SetImageRepo(root string) image.Repo {
	return image.InitRepository(root)
}
func InitRunc(root image.Repo, shuffix ...string) {
	repo = root.Path
	if len(shuffix) == 1 {
		stateDir, err := securejoin.SecureJoin(repo, shuffix[0])
		if err != nil {
			panic(err)
		}
		repo = stateDir
	}
	taskDir = filepath.Join(root.Path, "tasks")
}
func SetTaskDir(root string) {
	dir, err := securejoin.SecureJoin(root, "tasks")
	if err != nil {
		panic(err)

	}
	taskDir = dir
}

type action interface {
	Create()
	Run(rm ...bool)
	Restore()
}

type task struct {
	container *libcontainer.Container
	process   *libcontainer.Process
}

func (t *task) Rm() {
	if t.isEmpty() {
		return
	}
	t.container.Exec()

	t.process.Wait()
	t.container.Destroy()

	//defer t.container.Destroy()
	//t.process.Wait()
	//t.container.Destroy()

}
func (t *task) isEmpty() bool {
	return reflect.DeepEqual(t, &task{})
}

func (t *task) Create() {
	if t.isEmpty() {
		return
	}
	err := t.container.Start(t.process)
	if err != nil {
		fmt.Println("create", err)
		return
	}

}

func (t *task) Run(rm ...bool) {
	if t.isEmpty() {
		return
	}
	if len(rm) != 1 {
		rm = []bool{false}
	}
	t.container.Run(t.process)

	if rm[0] {
		t.process.Wait()
		t.container.Destroy()

	}
}

func (t *task) Restore() {}

func NewContainer(id string, opts ...NewContainerOpts) action {

	if repo == "" {
		fmt.Println("initRunc")
		return &task{}
	}

	stateDir, err := securejoin.SecureJoin(repo, id)
	if err != nil {
		fmt.Println("path", err)
		return &task{}
	}

	flist, _ := os.ReadDir(stateDir)
	if len(flist) == 0 {
		os.Remove(stateDir)
	}

	s := &specconv.CreateOpts{
		CgroupName: id,
	}
	s.Spec = defaultSpec()
	s.Spec.Linux.Seccomp = nil

	for _, o := range opts {
		o(s)
	}
	s.Spec.Process.Terminal = true
	config, err := specconv.CreateLibcontainerConfig(s)
	if err != nil {
		fmt.Println("specconv.CreateLibcontainerConfig", err)
		return &task{}
	}

	//fmt.Printf("%+v\n", s.Spec.Process)
	/*
		postStop := configs.NewFunctionHook(func(s *specs.State) error {
			err := mount.UnmountAll(filepath.Join(taskDir, id, "rootfs"), 0)
			os.WriteFile("/tmp/hahacs/umount.log", []byte(fmt.Sprintf("%v", err)), 0700)
			return nil
		})
	*/

	umountpath, err := exec.LookPath("umount")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(umountpath)
	rootfs := filepath.Join(taskDir, id, "rootfs")
	postStop := configs.NewCommandHook(configs.Command{
		Path: umountpath,
		Env:  s.Spec.Process.Env,
		Args: []string{umountpath, rootfs},
	})
	fmt.Println(postStop.Command)

	config.Hooks = configs.Hooks{configs.Poststop: configs.HookList{postStop}}

	container, err := libcontainer.Create(repo, id, config)
	if err != nil {
		fmt.Println("libcontainer.Create", err)
		return &task{}
	}
	process, err := newProcess(*s.Spec.Process)
	if err != nil {
		fmt.Println("newProcess", err)
		return &task{}
	}
	return &task{
		container: container,
		process:   process,
	}

}

func Start(id string) {
	container, err := libcontainer.Load(repo, id)
	if err != nil {
		fmt.Println("container.Load", err)
		return
	}
	status, err := container.Status()
	if err != nil {
		fmt.Println("container.Status", err)
		return
	}
	switch status {
	case libcontainer.Created:
		err := container.Exec()
		if err != nil {
			fmt.Println("start", err)
		}
		return
	case libcontainer.Stopped:
		fmt.Println("cannot start a container that has stopped")
		return
	case libcontainer.Running:
		fmt.Println("cannot start an already running container")
		return
	default:
		fmt.Printf("cannot start a container in the %s state", status)
		return
	}
}

func Delete(id string, force ...bool) {
	container, err := libcontainer.Load(repo, id)
	if err != nil {
		fmt.Println("container.Load", err)
		return
	}
	if len(force) != 1 {
		force = []bool{false}
	}

	status, err := container.Status()
	if err != nil {
		return
	}
	switch status {
	case libcontainer.Stopped:
		err := container.Destroy()
		if err != nil {
			fmt.Println(err)
		}

	case libcontainer.Created:
		killContainer(container)
	default:
		if force[0] {
			killContainer(container)
		}
		fmt.Printf("cannot delete container %s that is not stopped: %s\n", id, status)
	}

}
func killContainer(container *libcontainer.Container) error {
	_ = container.Signal(unix.SIGKILL)
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := container.Signal(unix.Signal(0)); err != nil {
			container.Destroy()
			return nil
		}
	}
	return errors.New("container init still running")
}

func Kill(id string, sig os.Signal) {
	container, err := libcontainer.Load(repo, id)
	if err != nil {
		fmt.Println("container.Load", err)
		return
	}
	container.Signal(sig)

}

func Exec(id string, cmd ...string) {
	container, err := libcontainer.Load(repo, id)
	if err != nil {
		fmt.Println("container.Load", err)
		return
	}

	status, err := container.Status()
	if err != nil {
		fmt.Println("container.Status", err)
		return
	}

	if status == libcontainer.Stopped {
		fmt.Println("cannot exec in a stopped container")
		return
	}
	if status == libcontainer.Paused {
		fmt.Println("cannot exec in a paused container (use --ignore-paused to override)")
		return
	}

	spec := defaultSpec()
	spec.Process.Terminal = false
	spec.Process.Args = cmd
	process, err := newProcess(*spec.Process)
	if err != nil {
		fmt.Println(err)
		return
	}
	process.Init = false
	err = container.Run(process)
	if err != nil {
		fmt.Println("exec", err)
		return
	}
}

func List() {

	s, err := getContainers()
	if err != nil {
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "ID\tPID\tIMAGE\tCOMMAND\tSTATUS\tCREATED\tOWNER\n")

	for _, item := range s {

		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			item.id,
			item.initProcessPid,
			item.image,
			item.command,
			item.status,
			item.create,
			item.owner,
		)

	}

	w.Flush()

}

type State struct {
	id             string
	initProcessPid int
	image          string
	command        string
	status         string
	create         string
	owner          string
}

func getContainers() ([]State, error) {
	list, err := os.ReadDir(repo)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var s []State
	for _, item := range list {
		if !item.IsDir() {
			continue
		}
		st, err := item.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		uid := st.Sys().(*syscall.Stat_t).Uid
		owner, err := user.LookupUid(int(uid))
		if err != nil {
			owner.Name = fmt.Sprintf("#%d", uid)
		}
		container, err := libcontainer.Load(repo, item.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "load container %s: %v\n", item.Name(), err)
			continue
		}
		status, err := container.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "status for %s: %v\n", item.Name(), err)
			continue
		}
		state, err := container.State()
		if err != nil {
			fmt.Fprintf(os.Stderr, "state for %s: %v\n", item.Name(), err)
			continue
		}
		pid := state.BaseState.InitProcessPid
		if status == libcontainer.Stopped {
			pid = 0
		}
		_, label := utils.Annotations(state.Config.Labels)
		s = append(s, State{
			id:             state.BaseState.ID,
			initProcessPid: pid,
			image:          label["image"],
			command:        label["command"],
			status:         status.String(),
			create:         time.Now().Local().Sub(state.Created.Local()).String(),
			owner:          owner.Name,
		})

	}
	return s, nil
}
