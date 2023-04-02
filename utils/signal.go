/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package utils

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var (
	terminalCh = make(chan os.Signal, 1)
	userCh     = make(chan os.Signal, 1)
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
			buf       []byte
			stackSize int
			startSize = math.MaxInt32
		)
		for len(buf) == stackSize {
			buf = make([]byte, startSize)
			stackSize = runtime.Stack(buf, true)
			startSize *= 2
		}
		fmt.Println(string(buf[:stackSize]))
	}
}

func Shutdown() {
	terminalCh <- syscall.SIGQUIT
}
