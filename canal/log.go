package canal

import (
	"fmt"
	"os"

	"github.com/siddontang/go-log/log"
)

type logger struct {
	*log.Logger
	label string
}

func newlogger(label string) *logger {
	stdout, _ := log.NewStreamHandler(os.Stdout)
	s := &logger{Logger: log.NewDefault(stdout)}
	s.label = label
	return s
}

func (l *logger) Info(args ...interface{}) {
	l.Logger.Output(2, log.LevelInfo, l.label+fmt.Sprint(args...))
}

func (l *logger) Infof(format string, args ...interface{}) {
	l.Output(2, log.LevelInfo, l.label+fmt.Sprintf(format, args...))
}
