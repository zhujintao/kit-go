package utils

import (
	"io"
	"log/slog"
	"os"
	"path"
	"time"
)

type Logger struct {
	*slog.Logger
}

var level *slog.LevelVar = &slog.LevelVar{}

// Debug = -4
//
// Info   = 0
//
// Warn   = 4
//
// Error  = 8
func SlogLevel(logLevel int) {
	level.Set(slog.Level(logLevel))
}
func Slog(logfile ...string) Logger {

	if len(logfile) == 1 {
		f, err := os.OpenFile(logfile[0], os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err == nil {
			io.MultiWriter(os.Stderr, f)
		}

	}

	l := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				s := a.Value.Any().(*slog.Source)
				s.File = path.Base(s.File)
			}
			if a.Key == slog.TimeKey {

				t := a.Value.Time()
				a.Value = slog.StringValue(t.Format(time.DateTime))

			}
			return a
		},
		Level: level,
	}))

	return Logger{l}
}
