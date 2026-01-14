package api

import (
	"context"

	"github.com/basenana/friday/core/logger"
	"github.com/basenana/friday/core/memory"
	"github.com/basenana/friday/core/types"
)

const (
	ctxMemoryKey   = "friday.ctx.memory"
	ctxSessionKey  = "friday.ctx.session"
	ctxResponseKey = "friday.ctx.response"
	ctxToolArgsKey = "friday.ctx.tool.args"
)

func SessionFromContext(ctx context.Context) *types.Session {
	m := ctx.Value(ctxSessionKey)
	if m == nil {
		return nil
	}
	return m.(*types.Session)
}

func MemoryFromContext(ctx context.Context) *memory.Memory {
	m := ctx.Value(ctxMemoryKey)
	if m == nil {
		return nil
	}
	return m.(*memory.Memory)
}

func ResponseFromContext(ctx context.Context) *Response {
	r := ctx.Value(ctxResponseKey)
	if r == nil {
		return nil
	}
	return r.(*Response)
}

func OverwriteToolArgsFromContext(ctx context.Context) map[string]string {
	s := ctx.Value(ctxToolArgsKey)
	if s == nil {
		return map[string]string{}
	}
	return s.(map[string]string)
}

type ContextOption func(ctx context.Context) context.Context

func NewContext(ctx context.Context, session *types.Session, options ...ContextOption) context.Context {
	// ensure session id
	s := ctx.Value(ctxSessionKey)
	if s == nil || s.(*types.Session).ID != session.ID {
		ctx = context.WithValue(ctx, ctxSessionKey, session)
	}

	for _, opt := range options {
		ctx = opt(ctx)
	}

	return ctx
}

func WithMemory(mem *memory.Memory) ContextOption {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, ctxMemoryKey, mem)
	}
}

func WithOverwriteToolArgs(overwrite map[string]string) ContextOption {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, ctxToolArgsKey, overwrite)
	}
}

func WithResponse(resp *Response) ContextOption {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, ctxResponseKey, resp)
	}
}

func SendEventToResponse(ctx context.Context, evt *types.Event, extraKV ...string) {
	if evt == nil {
		return
	}

	resp := ResponseFromContext(ctx)
	if resp == nil {
		logger.New("eventRecorder").Errorw("no response found in context.Context, event will be dropped", "evt", evt)
		return
	}

	SendEvent(resp, evt, extraKV...)
}
