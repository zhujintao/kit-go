package progress

const (
	escape = "\x1b"
	reset  = escape + "[0m"
	red    = escape + "[31m" //nolint:nolintlint,unused,varcheck
	green  = escape + "[32m"
)
