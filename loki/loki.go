package loki

import (
	"log/slog"
	"runtime"
	"time"
)

//loki("http:///v3/api/push").msg("aaaa","sse","xxxx")

type loki struct {
}

//多租房

func New(url string) *loki {
	return &loki{}
}

func (l *loki) Send(message string, args ...any) {
	var pc uintptr
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	pc = pcs[0]
	r := slog.NewRecord(time.Now(), slog.LevelInfo, message, pc)
	r.Add(args...)

}
