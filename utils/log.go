package utils

import (
	"log/slog"
	"os"
	"path"
	"time"
)

type Logger *slog.Logger

func Newlog() Logger {

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
	}))

	return l
}
