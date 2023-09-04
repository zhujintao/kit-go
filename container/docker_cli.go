package container

import (
	"context"
	"fmt"
	"io"
	"strings"

	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type dockerCli struct {
	*client.Client
}

func newDockerCli(ctxx context.Context) (*dockerCli, error) {
	ctx = ctxx
	c, _ := client.NewClientWithOpts(client.FromEnv)
	c.NegotiateAPIVersion(ctx)
	_, err := c.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &dockerCli{c}, nil

}

func (c *dockerCli) Create(ref, id string) {

	canonicalRef, _ := refdocker.ParseDockerRef(ref)

	cs, _ := c.ImageList(ctx, types.ImageListOptions{Filters: filters.NewArgs(filters.Arg("reference", canonicalRef.String()))})

	if len(cs) == 0 {
		out, err := c.ImagePull(ctx, ref, types.ImagePullOptions{})

		if err != nil {

			return
		}
		defer out.Close()
		io.Copy(io.Discard, out)
	}

	containerConfig := &container.Config{
		Image: ref,

		Cmd: strings.Fields("ls /dev"),
	}

	hostConfig := &container.HostConfig{Binds: []string{"/boot:/boot:ro"},
		NetworkMode: container.NetworkMode("default"),
	}

	resp, err := c.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, id)
	if err != nil {
		fmt.Println(err)
		return
	}

	if err := c.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(resp.ID)

}
