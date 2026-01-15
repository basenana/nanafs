package logger

import (
	fridaylogger "github.com/basenana/friday/core/logger"
	"go.uber.org/zap"
)

var (
	root *zap.SugaredLogger
)

func SetLogger(log *zap.SugaredLogger) {
	root = log
	fridaylogger.SetDefault(&fridayLogger{SugaredLogger: root.Named("friday")})
}

func NewLogger(name string) *zap.SugaredLogger {
	return root.Named(name)
}

func NewPluginLogger(name, jobID string) *zap.SugaredLogger {
	return NewLogger(name).With(zap.String("job", jobID))
}
