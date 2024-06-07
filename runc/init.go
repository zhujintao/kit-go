package runc

import (
	"os"

	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/cgroups/devices"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

func init() {

	if len(os.Args) > 1 && os.Args[1] == "init" {
		libcontainer.Init()
	}

}
