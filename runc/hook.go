package runc

import (
	"encoding/json"
	"os"
)

// CmdHookFn backcall must come first
var CmdHookFn func(state State) (exitCode int)

func cmdHook() {
	if len(os.Args) > 1 && os.Args[1] == "hook" {

		var state State
		json.NewDecoder(os.Stdin).Decode(&state)
		if CmdHookFn == nil {
			os.Exit(0)
		}
		exitCode := CmdHookFn(state)
		os.Exit(exitCode)

	}
}
