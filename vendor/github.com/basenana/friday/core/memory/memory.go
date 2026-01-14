package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/basenana/friday/core/logger"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/friday/core/types"
)

type Memory struct {
	ctxID     string
	copyTimes int

	session Session

	history []types.Message
	tools   []*tools.Tool
	mux     sync.Mutex

	tokens int64
	logger logger.Logger
}

func (m *Memory) History() []types.Message {
	m.mux.Lock()
	defer m.mux.Unlock()
	result := make([]types.Message, 0, len(m.history)+1)
	result = append(result, m.history...)
	return result
}

func (m *Memory) Modify(modifyMyMemory func(messages []types.Message) []types.Message) {
	m.mux.Lock()
	defer m.mux.Unlock()
	oldMemory := make([]types.Message, 0, len(m.history)+1)
	oldMemory = append(oldMemory, m.history...)
	m.history = modifyMyMemory(oldMemory)

	oldTokens := m.tokens
	m.tokens = 0
	for _, msg := range m.history {
		m.tokens += msg.FuzzyTokens()
	}
	m.logger.Infow("history modified", "beforeTokens", oldTokens, "newTokens", m.tokens)
}

func (m *Memory) AppendMessages(messages ...types.Message) {
	m.mux.Lock()
	defer m.mux.Unlock()

	for _, message := range messages {
		m.session.AppendMessage(context.TODO(), m.ctxID, &message)
		m.history = append(m.history, message)
		m.tokens += message.FuzzyTokens()
	}
	m.logger.Infow("append new messages", "fuzzyTokens", m.tokens, "ctxID", m.ctxID)
}

func (m *Memory) RunHook(ctx context.Context, hookName string) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	payload := &types.SessionPayload{
		ContextID: m.ctxID,
		History:   m.history,
	}
	err := m.session.RunHooks(ctx, hookName, payload)
	if err != nil {
		m.logger.Errorw("failed to run hook", "hookName", hookName, "err", err)
		return err
	}

	var newTokens int64
	for _, message := range payload.History {
		newTokens += message.FuzzyTokens()
	}
	m.tokens = newTokens
	m.history = payload.History
	return nil
}

func (m *Memory) RunBeforeModelHook(ctx context.Context) error {
	return m.RunHook(ctx, types.SessionHookBeforeModel)
}

func (m *Memory) RunAfterModelHook(ctx context.Context) error {
	return m.RunHook(ctx, types.SessionHookAfterModel)
}

func (m *Memory) Scratchpad() tools.Scratchpad {
	return m.session.Scratchpad()
}

func (m *Memory) Session() *types.Session {
	return m.session.Session()
}

func (m *Memory) Tools() []*tools.Tool {
	if m.tools != nil {
		return m.tools
	}

	memoryTools := make([]*tools.Tool, 0)
	if v := m.session.Scratchpad(); v != nil {
		memoryTools = append(memoryTools, tools.ScratchpadReadTools(v)...)
	}
	m.tools = memoryTools
	return m.tools
}

func (m *Memory) Tokens() int64 {
	return m.tokens
}

func (m *Memory) Copy() *Memory {
	m.mux.Lock()
	defer m.mux.Unlock()

	m.copyTimes += 1
	mid := fmt.Sprintf("%s.%d", m.ctxID, m.copyTimes)
	nm := Memory{
		ctxID:   mid,
		session: m.session,
		tools:   m.tools,
		mux:     sync.Mutex{},
		tokens:  m.tokens,
		logger:  m.logger,
	}

	nm.history = make([]types.Message, len(m.history))
	for i, msg := range m.history {
		nm.history[i] = msg
	}
	return &nm
}

type OptionSetter func(*Memory)

func NewEmpty(ctxID string, setters ...OptionSetter) *Memory {
	mem := &Memory{
		ctxID:   ctxID,
		session: newLimitedSession(ctxID, 20),
		history: make([]types.Message, 0, 10),
		logger:  logger.New("memory"),
	}

	for _, setter := range setters {
		setter(mem)
	}
	return mem
}

func New(session Session, setters ...OptionSetter) *Memory {
	mem := &Memory{
		ctxID:   session.ID(),
		session: session,
		history: make([]types.Message, 0, 10),
		logger:  logger.New("memory"),
	}

	for _, setter := range setters {
		setter(mem)
	}

	mem.history = session.History(context.TODO())
	return mem
}

func WithUnlimitedSession() OptionSetter {
	return func(m *Memory) {
		m.session = newLimitedSession(m.ctxID, -1)
	}
}

func WithHistory(history ...types.Message) OptionSetter {
	return func(mem *Memory) {
		mem.history = history
	}
}
