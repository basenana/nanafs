package logger

import (
	flogger "github.com/basenana/friday/core/logger"
	"go.uber.org/zap"
)

type fridayLogger struct {
	logger *zap.SugaredLogger
}

func (f fridayLogger) Named(name string) flogger.Logger {
	return fridayLogger{f.logger.Named(name)}
}

func (f fridayLogger) With(keysAndValues ...interface{}) flogger.Logger {
	return fridayLogger{logger: f.logger.With(keysAndValues...)}
}

func (f fridayLogger) Info(args ...interface{}) {
	f.logger.Info(args...)
}

func (f fridayLogger) Warn(args ...interface{}) {
	f.logger.Warn(args...)
}

func (f fridayLogger) Error(args ...interface{}) {
	f.logger.Error(args...)
}

func (f fridayLogger) Infof(template string, args ...interface{}) {
	f.logger.Infof(template, args...)
}

func (f fridayLogger) Warnf(template string, args ...interface{}) {
	f.logger.Warnf(template, args...)
}

func (f fridayLogger) Errorf(template string, args ...interface{}) {
	f.logger.Errorf(template, args...)
}

func (f fridayLogger) Infow(msg string, keysAndValues ...interface{}) {
	f.logger.Infow(msg, keysAndValues...)
}

func (f fridayLogger) Warnw(msg string, keysAndValues ...interface{}) {
	f.logger.Warnw(msg, keysAndValues...)
}

func (f fridayLogger) Errorw(msg string, keysAndValues ...interface{}) {
	f.logger.Errorw(msg, keysAndValues...)
}

var _ flogger.Logger = &fridayLogger{}
