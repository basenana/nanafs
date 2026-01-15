package logger

import (
	fridaylogger "github.com/basenana/friday/core/logger"
	"go.uber.org/zap"
)

type fridayLogger struct {
	*zap.SugaredLogger
}

func (l *fridayLogger) With(keysAndValues ...interface{}) fridaylogger.Logger {
	return &fridayLogger{l.SugaredLogger.With(keysAndValues...)}
}

var _ fridaylogger.Logger = (*fridayLogger)(nil)
