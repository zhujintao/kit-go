package container

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/go-cni"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type containedCli struct {
	*containerd.Client
}

func newContainedCli(ctxx context.Context, namespace ...string) (*containedCli, error) {
	ns := "default"
	if len(namespace) == 1 {
		ns = namespace[1]
	}

	ctx = ctxx
	c, err := containerd.New(defaults.DefaultAddress, containerd.WithTimeout(1*time.Second))
	if err != nil {
		return nil, err
	}
	ctx = namespaces.WithNamespace(ctx, ns)
	return &containedCli{c}, nil

}

func (c *containedCli) Create(ref, id string) {

	defer c.Close()

	canonicalRef, _ := refdocker.ParseDockerRef(ref)
	cs, err := c.ListImages(ctx, "name=="+canonicalRef.String())
	if err != nil {
		return
	}

	var image containerd.Image
	if len(cs) == 0 {
		image, _ = c.Pull(ctx, ref, containerd.WithPullUnpack)
	} else {
		image = cs[0]
	}

	var (
		opts  []oci.SpecOpts
		cOpts []containerd.NewContainerOpts
		spec  containerd.NewContainerOpts
		//network cni.CNI
	)

	//base setting
	opts = append(opts, oci.WithDefaultSpec(), oci.WithDefaultUnixDevices)
	opts = append(opts, oci.WithImageConfig(image))
	opts = append(opts, oci.WithHostname(id))

	//network setting

	cniOpts := []cni.Opt{cni.WithPluginDir([]string{"/usr/libexec/cni", "/opt/cni/bin"})}
	cniOpts = append(cniOpts, cni.WithConfListFile("/etc/cni/net.d/nerdctl-vlan10.conflist"))

	var hooks oci.SpecOpts
	hooks = func(ctx context.Context, _ oci.Client, c *containers.Container, s *specs.Spec) error {
		if s.Hooks == nil {
			s.Hooks = &specs.Hooks{}
		}
		crArg := []string{"xxxx", "bbbb", "xxxx"}
		s.Hooks.CreateRuntime = append(s.Hooks.CreateRuntime, specs.Hook{
			Path: "/root/go/src/curvekit/hook.sh",
			Args: crArg,
			Env:  os.Environ(),
		})
		return nil
	}

	opts = append(opts, hooks)

	opts = append(opts,
		//oci.WithHostNamespace(specs.NetworkNamespace), // host network
		oci.WithHostHostsFile,
		oci.WithHostResolvconf,
	)
	//	opts = append(opts,oci.WithMounts([]specs.Mount{{Type: "cgroup", Source: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"ro", "nosuid", "noexec", "nodev"}},}))
	// oci.WithAnnotations(map[string]string{"haha": "xxxx"})
	cOpts = append(cOpts,
		containerd.WithImage(image),
		containerd.WithImageConfigLabels(image),
		containerd.WithSnapshotter(""),
	)

	cOpts = append(cOpts, containerd.WithNewSnapshot(id, image,
		snapshots.WithLabels(map[string]string{})))

	opts = append(opts, oci.WithProcessArgs(strings.Fields("ifconfig ")...))

	var s specs.Spec
	spec = containerd.WithSpec(&s, opts...)
	cOpts = append(cOpts, spec)
	container, err := c.NewContainer(ctx, id, cOpts...)
	if err != nil {
		fmt.Println(err)
		return
	}
	var ioCreator cio.Creator = cio.NullIO
	ioCreator = cio.NewCreator(cio.WithStreams(os.Stdin, os.Stdout, os.Stderr))
	//ioCreator = cio.LogFile("/tmp/" + id)
	tk, err := container.NewTask(ctx, ioCreator)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(tk)
	tk.Start(ctx)
}
