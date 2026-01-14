package logger

type Logger interface {
	With(keysAndValues ...interface{}) Logger

	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})

	Infof(template string, args ...interface{})
	Warnf(template string, args ...interface{})
	Errorf(template string, args ...interface{})

	Infow(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
}
