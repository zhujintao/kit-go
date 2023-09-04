package container

import (
	"context"

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

func (c *dockerCli) Create(ref, id string) {}
