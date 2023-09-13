package container

import (
	"context"
	"fmt"
)

var (
	ctx  context.Context
	name string
	ref  string
)

type Client interface {
	Create(ref, id string)
	Image(url, name string) params
}

type State struct {
	Status  string
	Running bool
	Pid     int
}

type params interface {
	Volume(volume string) params
	Entrypoint(command ...string) State // start container witch command
}

func newClient() (params, error) {

	/*
		if c, err := newContainedCli(ctx); err == nil {
			return c, nil
		}
	*/
	if c, err := newDockerCli(); err == nil {
		return c, nil
	}

	return nil, fmt.Errorf("no available client")
}

// option ref image url
//
// option name container id and hostname
func Image(image string, ContainerName ...string) params {

	if len(ContainerName) != 1 {
		ContainerName = []string{"haha"}
	}

	name = ContainerName[0]
	ref = image

	cli, err := newClient()
	if err != nil {
		return nil
	}

	return cli
}
