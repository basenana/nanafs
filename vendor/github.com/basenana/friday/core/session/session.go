package session

import (
	"sync"
	"time"

	"github.com/basenana/friday/core/fs"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/types"
)

type Session struct {
	ID        string
	Root      *Session
	Parent    *Session
	Children  []*Session
	History   []types.Message
	Workdir   fs.FileSystem
	CreatedAt time.Time

	compactThreshold int64

	hooks []Hook
	llm   openai.Client
	mu    sync.RWMutex
}

func New(id string, llm openai.Client, options ...Option) *Session {
	s := &Session{
		ID:               id,
		History:          make([]types.Message, 0, 10),
		compactThreshold: CompactThreshold,
		hooks:            make([]Hook, 0),
		CreatedAt:        time.Now(),
		llm:              llm,
	}
	s.Root = s

	for _, option := range options {
		option(s)
	}

	if s.Workdir == nil {
		s.Workdir = fs.NewInMemory()
	}
	return s
}

func (s *Session) Fork() *Session {
	s.mu.Lock()
	fork := &Session{
		ID:               types.NewID(),
		Root:             s.Root,
		Parent:           s,
		History:          make([]types.Message, len(s.History)),
		Workdir:          s.Workdir,
		CreatedAt:        time.Now(),
		compactThreshold: s.compactThreshold,
		hooks:            s.hooks,
		llm:              s.llm,
	}
	copy(fork.History, s.History)
	s.Children = append(s.Children, fork)
	s.mu.Unlock()

	return fork
}

func (s *Session) AppendMessage(msgList ...*types.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, msg := range msgList {
		s.History = append(s.History, *msg)
	}
}

func (s *Session) Tokens() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var total int64
	for _, msg := range s.History {
		total += msg.FuzzyTokens()
	}
	return total
}

type Option func(*Session)

func WithHistory(messages ...types.Message) Option {
	return func(s *Session) {
		s.History = messages
	}
}

func WithHooks(hooks ...Hook) Option {
	return func(s *Session) {
		s.hooks = append(s.hooks, hooks...)
	}
}

func WithWorkdirFS(wfs fs.FileSystem) Option {
	return func(s *Session) {
		s.Workdir = wfs
	}
}

func WithCompactThreshold(ct int64) Option {
	return func(s *Session) {
		s.compactThreshold = ct
	}
}
