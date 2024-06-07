package runc

import (
	"log/slog"
	"os"
	"time"
)

var log *slog.Logger = slog.New(slog.NewTextHandler(
	os.Stdout,
	&slog.HandlerOptions{AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key != slog.TimeKey {
				return a
			}
			t := a.Value.Time()
			a.Value = slog.StringValue(t.Format(time.DateTime))
			return a
		},
	}))
