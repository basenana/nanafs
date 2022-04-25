package utils

import (
	"context"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"net/http"
)

const (
	apiTraceContextKey = "api.trace"
)

func NewApiContext(r *http.Request) context.Context {
	return context.WithValue(r.Context(), apiTraceContextKey, uuid.New().String())
}

func ContextLog(ctx context.Context, log *zap.SugaredLogger) *zap.SugaredLogger {
	traceID := ""
	traceObj := ctx.Value(apiTraceContextKey)
	if traceObj != nil {
		traceID = traceObj.(string)
	}
	return log.With(zap.String("trace", traceID))
}
