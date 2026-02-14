package session

import (
	"context"

	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/friday/core/types"
)

// HookHandler is a function that can be registered to run at specific points in the session lifecycle.
// The req parameter allows hooks to access and modify the request before/after LLM calls.
type HookHandler func(ctx context.Context, sess *Session, req openai.Request) error

type Hook interface{}

type BeforeAgentHook interface {
	BeforeAgent(ctx context.Context, sess *Session, req AgentRequest) error
}

type BeforeModelHook interface {
	BeforeModel(ctx context.Context, sess *Session, req openai.Request) error
}

type AfterModelHook interface {
	AfterModel(ctx context.Context, sess *Session, req openai.Request, apply *openai.Apply) error
}

type AgentRequest interface {
	GetUserMessage() string
	SetUserMessage(msg string)
	GetTools() []*tools.Tool
	AppendTools(...*tools.Tool)
}

type HookPayload struct {
	ModelRequest openai.Request
	ModelApply   *openai.Apply
	AgentRequest AgentRequest
	Messages     []types.Message
}

func (s *Session) RegisterHook(handler Hook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks = append(s.hooks, handler)
}

func (s *Session) CleanHooks() {
	s.hooks = nil
}

func (s *Session) RunHooks(ctx context.Context, hookName string, payload HookPayload) error {
	if len(s.hooks) == 0 {
		return nil
	}

	switch hookName {
	case types.SessionHookBeforeAgent:

		for _, hook := range s.hooks {
			handler, ok := hook.(BeforeAgentHook)
			if !ok {
				continue
			}
			if err := handler.BeforeAgent(ctx, s, payload.AgentRequest); err != nil {
				return err
			}
		}

	case types.SessionHookBeforeModel:
		if err := s.autoCompactHistory(ctx, payload.ModelRequest); err != nil {
			return err
		}

		for _, hook := range s.hooks {
			handler, ok := hook.(BeforeModelHook)
			if !ok {
				continue
			}
			if err := handler.BeforeModel(ctx, s, payload.ModelRequest); err != nil {
				return err
			}
		}

	case types.SessionHookAfterModel:

		for _, hook := range s.hooks {
			handler, ok := hook.(AfterModelHook)
			if !ok {
				continue
			}
			if err := handler.AfterModel(ctx, s, payload.ModelRequest, payload.ModelApply); err != nil {
				return err
			}
		}

	}

	return nil
}
