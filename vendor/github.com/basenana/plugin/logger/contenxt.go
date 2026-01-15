package logger

import (
	"context"

	"go.uber.org/zap"
)

var (
	logKey = "ctx.logger"
)

func FromContext(ctx context.Context) *zap.SugaredLogger {
	loggerRaw := ctx.Value(logKey)

	if loggerRaw == nil {
		return NewLogger("default")
	}

	logger, ok := loggerRaw.(*zap.SugaredLogger)
	if !ok {
		log := NewLogger("default")
		log.Warnw("unable to cast logger from context")
		return log
	}

	return logger
}

func IntoContext(ctx context.Context, logger *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, logKey, logger)
}
