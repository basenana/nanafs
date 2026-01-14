package logger

import (
	"go.uber.org/zap"
)

type zapLogger struct {
	*zap.SugaredLogger
}

func NewZapLogger(l *zap.SugaredLogger) Logger {
	return &zapLogger{l}
}

func (l *zapLogger) With(keysAndValues ...interface{}) Logger {
	newLogger := l.SugaredLogger.With(keysAndValues...)
	return &zapLogger{newLogger}
}

func (l *zapLogger) Info(args ...interface{}) {
	l.SugaredLogger.Info(args...)
}

func (l *zapLogger) Warn(args ...interface{}) {
	l.SugaredLogger.Warn(args...)
}

func (l *zapLogger) Error(args ...interface{}) {
	l.SugaredLogger.Error(args...)
}

func (l *zapLogger) Infof(template string, args ...interface{}) {
	l.SugaredLogger.Infof(template, args...)
}

func (l *zapLogger) Warnf(template string, args ...interface{}) {
	l.SugaredLogger.Warnf(template, args...)
}

func (l *zapLogger) Errorf(template string, args ...interface{}) {
	l.SugaredLogger.Errorf(template, args...)
}

func (l *zapLogger) Infow(msg string, keysAndValues ...interface{}) {
	l.logw(msg, keysAndValues, l.SugaredLogger.Infow)
}

func (l *zapLogger) Warnw(msg string, keysAndValues ...interface{}) {
	l.logw(msg, keysAndValues, l.SugaredLogger.Warnw)
}

func (l *zapLogger) Errorw(msg string, keysAndValues ...interface{}) {
	l.logw(msg, keysAndValues, l.SugaredLogger.Errorw)
}

func (l *zapLogger) logw(msg string, keysAndValues []interface{}, fn func(string, ...interface{})) {
	fields := make([]interface{}, 0, len(keysAndValues))
	for _, v := range keysAndValues {
		fields = append(fields, convertToZapField(v))
	}
	fn(msg, fields...)
}

func convertToZapField(v interface{}) interface{} {
	if err, ok := v.(error); ok {
		return zap.Error(err)
	}
	return v
}
