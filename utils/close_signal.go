package utils

import (
	"os"
	"os/signal"
	"syscall"
)

type sig struct {
	stop chan os.Signal
}

func SignalNotify() *sig {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop,
		os.Kill,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	return &sig{stop}
}
func (s *sig) Close(f func()) {

	go func() {
		<-s.stop
		f()
	}()

}
