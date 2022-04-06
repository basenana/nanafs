package logger

import "go.uber.org/zap"

var (
	logger *zap.Logger
	root   *zap.SugaredLogger
)

func InitLogger() {
	logger, _ = zap.NewProduction()
	root = logger.Sugar()
}

func Sync() {
	_ = logger.Sync()
}

func NewLogger(name string) *zap.SugaredLogger {
	return root.Named(name)
}
