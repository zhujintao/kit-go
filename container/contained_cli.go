package container

import (
	"context"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
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

	c.ListImages(ctx, ref)

	defer c.Close()

}
