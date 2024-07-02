package loki

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"runtime"
	"sync"

	"github.com/prometheus/common/model"
	"github.com/zhujintao/kit-go/utils/buffer"
)

type Level int

const (
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
)

type LokiHandler struct {
	url      string
	opts     slog.HandlerOptions
	mu       *sync.Mutex
	jobLabel string
}

// jobLabel is label job id
//
// logLevel default info
type Config struct {
	URL      string
	JobLabel string
	Level    Level
}

// jobLabel is label job id
//
// logLevel default info
//
// -4 Debug
//
// 0  Info
//
// 4  Warn
//
// 8  Error

// NewLokiHandler creates a [LokiHandler] that writes to loki server,
func NewLokiHandler(config Config) *LokiHandler {

	var level *slog.LevelVar = &slog.LevelVar{}
	level.Set(slog.Level(config.Level))
	return &LokiHandler{
		url:      config.URL,
		mu:       &sync.Mutex{},
		jobLabel: config.JobLabel,
		opts:     slog.HandlerOptions{Level: level},
	}

}

func (l *LokiHandler) Handle(_ context.Context, r slog.Record) error {

	var buf buffer.Buffer = *buffer.New()
	lbs := make(model.LabelSet, r.NumAttrs())

	if !r.Time.IsZero() {
		buf.WriteString(" ")
		buf.WriteString("time")
		buf.WriteByte('=')
		buf.WriteString(r.Time.Format("2006-01-02 15:04:05.000"))

	}

	buf.WriteString(" ")
	buf.WriteString("level")
	buf.WriteByte('=')
	buf.WriteString(r.Level.String())
	lbs[model.LabelName("level")] = model.LabelValue(r.Level.String())

	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	buf.WriteString(" ")
	buf.WriteString("source")
	buf.WriteByte('=')
	buf.WriteString(fmt.Sprintf("%s:%d", path.Base(f.File), f.Line))

	buf.WriteString(" " + "msg=" + r.Message)

	r.Attrs(func(a slog.Attr) bool {

		buf.WriteString(" ")
		buf.WriteString(string(a.Key))
		buf.WriteByte('=')
		buf.WriteString(a.Value.String())
		lbs[model.LabelName(a.Key)] = model.LabelValue(a.Value.String())
		return true
	})
	if l.jobLabel != "" {
		lbs[model.LabelName("job")] = model.LabelValue(l.jobLabel)
	}

	return nil
}

func (l *LokiHandler) Enabled(_ context.Context, level slog.Level) bool {

	minLevel := slog.LevelInfo
	if l.opts.Level != nil {
		minLevel = l.opts.Level.Level()
	}
	return level >= minLevel
}

func (l *LokiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	fmt.Println("WithAttrs", attrs)
	return l
}

func (l *LokiHandler) WithGroup(name string) slog.Handler {
	fmt.Println("WithGroup", name)
	return l
}
