package summarize

import (
	"context"

	"github.com/basenana/friday/core/agents"
	"github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/logger"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/session"
	"github.com/basenana/friday/core/types"
)

type Agent struct {
	react  agents.Agent
	llm    openai.Client
	option Option
	logger logger.Logger
}

func (a *Agent) Chat(ctx context.Context, req *api.Request) *api.Response {
	userMessage := req.UserMessage
	if userMessage == "" {
		userMessage = DEFAULT_USER_MESSAGE
	}

	sess := req.Session
	if sess == nil {
		sess = session.New(types.NewID(), a.llm)
	}

	return a.react.Chat(ctx, &api.Request{
		Session:     sess,
		UserMessage: userMessage,
	})
}

func New(llm openai.Client, option Option) *Agent {
	if option.SystemPrompt == "" {
		option.SystemPrompt = DEFAULT_SUMMARIZE_PROMPT
	}
	return &Agent{
		react:  agents.New(llm, agents.Option{SystemPrompt: option.SystemPrompt}),
		llm:    llm,
		option: option,
		logger: logger.New("summarize"),
	}
}

type Option struct {
	SystemPrompt string
}
