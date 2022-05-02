package utils

import (
	"context"
	"fmt"
	"runtime/trace"
)

func TraceTask(ctx context.Context, taskName string) (context.Context, func()) {
	ctx, t := trace.NewTask(ctx, taskName)
	return ctx, func() {
		t.End()
	}
}

func TraceRegion(ctx context.Context, message string, args ...interface{}) func() {
	t := trace.StartRegion(ctx, fmt.Sprintf(message, args...))
	return func() {
		t.End()
	}
}
