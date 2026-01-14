package agents

import (
	"context"

	"github.com/basenana/friday/core/api"
)

type Agent interface {
	Chat(ctx context.Context, req *api.Request) *api.Response
}
