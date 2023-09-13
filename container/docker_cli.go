package container

import (
	"context"
	"fmt"
	"io"
	"strings"

	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

type dockerCli struct {
	*client.Client
}

var containerConfig *container.Config = &container.Config{}
var hostConfig *container.HostConfig = &container.HostConfig{NetworkMode: container.NetworkMode("none")}

func newDockerCli() (*dockerCli, error) {
	ctx = context.Background()
	c, _ := client.NewClientWithOpts(client.FromEnv)
	c.NegotiateAPIVersion(ctx)
	_, err := c.Ping(ctx)
	if err != nil {
		return nil, err
	}

	err = image(c)
	if err != nil {
		return nil, err
	}

	return &dockerCli{c}, nil

}

func image(c *client.Client) error {

	canonicalRef, _ := refdocker.ParseDockerRef(ref)
	cs, _ := c.ImageList(ctx, types.ImageListOptions{Filters: filters.NewArgs(filters.Arg("reference", canonicalRef.String()))})

	if len(cs) == 0 {
		out, err := c.ImagePull(ctx, ref, types.ImagePullOptions{})
		if err != nil {
			return err
		}
		defer out.Close()
		io.Copy(io.Discard, out)
	}
	containerConfig.Image = ref
	return nil
}

func (c *dockerCli) Volume(volume string) params {

	hostConfig.Binds = append(hostConfig.Binds, volume)

	return c
}

func (c *dockerCli) Entrypoint(command ...string) State {
	containerConfig.Hostname = name
	if len(command) == 1 {
		containerConfig.Entrypoint = strings.Fields(command[0])
	}
	resp, err := c.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, name)
	if err != nil {
		fmt.Println(err)
		return State{Status: err.Error()}
	}

	err = c.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		fmt.Println(err)
		return State{Status: err.Error()}
	}
	container, err := c.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return State{Status: err.Error(), Running: container.State.Running}
	}
	return State{Running: container.State.Running, Pid: container.State.Pid}

}
