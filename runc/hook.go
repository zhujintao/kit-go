package runc

import (
	"encoding/json"
	"os"
)

var CmdHookFn func(state State)

func cmdHook() error {

	var state State
	json.NewDecoder(os.Stdin).Decode(&state)
	CmdHookFn(state)
	return nil

}
