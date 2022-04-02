package utils

import (
	"os"
	"os/signal"
	"syscall"
)

var (
	terminalCh = make(chan os.Signal)
	userCh     = make(chan os.Signal)
)

func init() {
	signal.Notify(terminalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	signal.Notify(userCh, syscall.SIGUSR1, syscall.SIGUSR2)
}

func HandleTerminalSignal() chan struct{} {
	ch := make(chan struct{})

	go func() {
		<-terminalCh
		close(terminalCh)
		close(ch)
	}()

	return ch
}

func Shutdown() {
	terminalCh <- syscall.SIGQUIT
}
