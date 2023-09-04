package container

import (
	"context"
	"fmt"
)

var ctx context.Context

type Client interface {
	Create(ref, id string)
}

func NewClient(ctx context.Context) (Client, error) {

	if c, err := newContainedCli(ctx); err == nil {
		return c, nil
	}

	if c, err := newDockerCli(ctx); err == nil {
		return c, nil
	}

	return nil, fmt.Errorf("no available client")
}
