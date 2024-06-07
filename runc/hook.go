package runc

import (
	"encoding/json"
	"os"
)

var CmdHookFn func(state State) error

func cmdHook() {
	if len(os.Args) > 1 && os.Args[1] == "hook" {

		var state State
		json.NewDecoder(os.Stdin).Decode(&state)
		if CmdHookFn == nil {
			os.Exit(0)
		}
		err := CmdHookFn(state)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
}
