package utils

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var (
	terminalCh = make(chan os.Signal)
	userCh     = make(chan os.Signal)
)

func init() {
	signal.Notify(terminalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	signal.Notify(userCh, syscall.SIGUSR1, syscall.SIGUSR2)

	go handlerUserSignal()
}

func HandleTerminalSignal() chan struct{} {
	ch := make(chan struct{})

	go func() {
		<-terminalCh
		close(ch)
		<-terminalCh
		os.Exit(2)
	}()

	return ch
}

func handlerUserSignal() {
	s := <-userCh
	if s == syscall.SIGUSR1 {
		var (
			buf = make([]byte, 2048)
		)
		n := runtime.Stack(buf, true)
		fmt.Println(string(buf[:n]))
	}
}

func Shutdown() {
	terminalCh <- syscall.SIGQUIT
}
