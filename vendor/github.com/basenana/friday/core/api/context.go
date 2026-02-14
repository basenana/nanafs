package api

import (
	"context"

	"github.com/basenana/friday/core/session"
)

const (
	ctxSessionKey  = "friday.ctx.session"
	ctxResponseKey = "friday.ctx.response"
	ctxToolArgsKey = "friday.ctx.tool.args"
	ctxEventBusKey = "friday.ctx.eventbus"
)

func SessionFromContext(ctx context.Context) *session.Session {
	s := ctx.Value(ctxSessionKey)
	if s == nil {
		return nil
	}
	return s.(*session.Session)
}
