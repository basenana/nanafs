package logger

import (
	"os"
	"sync"
)

var (
	rootLoggerInstance Logger
	rootLoggerMu       sync.Mutex
)

func New(name string) Logger {
	return Root().Named(name)
}

func Root() Logger {
	rootLoggerMu.Lock()
	defer rootLoggerMu.Unlock()
	if rootLoggerInstance == nil {
		rootLoggerInstance = &defaultLogger{name: "default", w: os.Stdout}
	}
	return rootLoggerInstance
}

func SetRoot(l Logger) {
	rootLoggerMu.Lock()
	defer rootLoggerMu.Unlock()
	rootLoggerInstance = l
}
