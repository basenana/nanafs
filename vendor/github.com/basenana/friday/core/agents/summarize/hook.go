package summarize

import (
	"context"

	"github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/session"
)

type Hook struct {
	llm              openai.Client
	compactThreshold int64
}

var _ session.BeforeModelHook = &Hook{}

func (h Hook) BeforeModel(ctx context.Context, sess *session.Session, req openai.Request) error {
	if sess.Tokens() < h.compactThreshold {
		return nil
	}

	newSession := sess.Fork()
	newSession.CleanHooks()
	agt := New(h.llm, Option{})
	stream := agt.Chat(ctx, &api.Request{Session: newSession})
	abstract, err := api.ReadAllContent(ctx, stream)
	if err != nil {
		return err
	}

	sess.History = session.RebuildHistoryWithAbstract(sess.History, abstract)
	req.SetHistory(sess.History)
	return nil
}

func NewCompactHook(llm openai.Client, compactThreshold int64) *Hook {
	return &Hook{llm: llm, compactThreshold: compactThreshold}
}
