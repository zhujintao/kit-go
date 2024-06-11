package runc

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/opencontainers/runc/libcontainer/specconv"
)

// CmdHookFn backcall must come first
var postStartHookFn func(state State) error

func postStartHook() {
	if len(os.Args) > 1 && os.Args[1] == "hook" {

		var state State
		json.NewDecoder(os.Stdin).Decode(&state)
		if postStartHookFn == nil {
			os.Exit(0)
		}
		err := postStartHookFn(state)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		os.Exit(0)

	}
}

func WithContainerPoststartHook(fn func(state State) error) createOpts {

	return func(c *specconv.CreateOpts) error {
		postStartHookFn = fn
		return nil
	}

}
