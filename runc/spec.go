package runc

import (
	"runtime"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
)

func defaultSpec() *specs.Spec {

	tmpl, err := generate.New(runtime.GOOS)
	if err != nil {
		return nil
	}
	tmpl.SetProcessArgs([]string{""})
	tmpl.ClearAnnotations()
	tmpl.SetProcessTerminal(true)
	tmpl.SetHostname(generateID())
	tmpl.Config.Linux.Seccomp = nil

	return tmpl.Config
}
