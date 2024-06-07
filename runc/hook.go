package runc

import (
	"encoding/json"
	"os"
)

var CmdHookFn func(state State) int

func cmdHook() {
	if len(os.Args) > 1 && os.Args[1] == "hook" {

		var state State
		json.NewDecoder(os.Stdin).Decode(&state)
		if CmdHookFn == nil {
			os.Exit(0)
		}
		code := CmdHookFn(state)
		os.Exit(code)

	}
}
