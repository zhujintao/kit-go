package log

import (
	"context"
	"log/slog"
	"runtime"
)

type customHandler struct {
	slog.Handler
}

func (h *customHandler) Handle(ctx context.Context, r slog.Record) error {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	a := &slog.Source{
		Function: f.Function,
		File:     f.File,
		Line:     f.Line,
	}
	r.AddAttrs(slog.Any(slog.SourceKey, a))

	return h.Handler.Handle(ctx, r)
}

func SlogDefaultWithId(id string) slog.Handler {
	baseHandler := slog.Default().Handler().WithAttrs([]slog.Attr{slog.String("id", id)})
	return &customHandler{Handler: baseHandler}
}
