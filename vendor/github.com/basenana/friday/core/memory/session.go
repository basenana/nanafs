package memory

import (
	"context"

	"github.com/basenana/friday/core/tools"
	"github.com/basenana/friday/core/types"
)

type Session interface {
	ID() string
	History(ctx context.Context) []types.Message
	AppendMessage(cte context.Context, ctxID string, message *types.Message)
	RunHooks(ctx context.Context, hookName string, payload *types.SessionPayload) error
	Scratchpad() tools.Scratchpad
	Session() *types.Session
}

type limitedSession struct {
	id           string
	historyLimit int
	scratchpad   tools.Scratchpad
}

func newLimitedSession(id string, limited int) Session {
	return &limitedSession{id: id, historyLimit: limited, scratchpad: tools.NewInMemoryScratchpad()}
}

func (e *limitedSession) ID() string {
	return e.id
}

func (e *limitedSession) History(ctx context.Context) []types.Message {
	return make([]types.Message, 0, 4)
}

func (e *limitedSession) AppendMessage(ctx context.Context, ctxID string, message *types.Message) {
	return
}

func (e *limitedSession) RunHooks(ctx context.Context, hookName string, payload *types.SessionPayload) error {
	if hookName != types.SessionHookBeforeModel {
		return nil
	}

	if e.historyLimit > 0 && len(payload.History) > e.historyLimit {
		cutAt := len(payload.History) - e.historyLimit
		payload.History = payload.History[cutAt:]
	}
	return nil
}

func (e *limitedSession) Scratchpad() tools.Scratchpad {
	return e.scratchpad
}

func (e *limitedSession) Session() *types.Session {
	return &types.Session{ID: e.id}
}
