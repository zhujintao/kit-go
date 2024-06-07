package runc

import (
	"encoding/json"
	"os"

	"github.com/opencontainers/runtime-spec/specs-go"
)

var CmdHookFn func(spec *specs.State)

func cmdHook() error {

	var state *specs.State
	json.NewDecoder(os.Stdin).Decode(&state)
	CmdHookFn(state)
	return nil

}
