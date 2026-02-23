package friday

import (
	"context"
	"fmt"
	"path"

	"github.com/basenana/friday/core/agents"
	"github.com/basenana/friday/core/agents/summarize"
	"github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/planning"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/session"
	"github.com/basenana/friday/core/subagents"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/indexer"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
)

type Session = session.Session

type Friday struct {
	fs        *core.FileSystem
	llm       openai.Client
	agt       agents.Agent
	indexer   indexer.Indexer
	namespace string
	store     SessionStore
}

func NewFriday(fs *core.FileSystem, llm openai.Client, indexer indexer.Indexer, store SessionStore) *Friday {
	f := &Friday{
		fs:        fs,
		llm:       llm,
		indexer:   indexer,
		namespace: fs.Namespace(),
		store:     store,
	}
	agt := agents.New(llm, agents.Option{SystemPrompt: DEFAULT_SYS_PROMPT, MaxLoopTimes: 20, Tools: f.Tools()})
	f.agt = agt
	return f
}

func (f *Friday) NewSession() (*session.Session, error) {
	sessID := uuid.New().String()
	sess := session.New(sessID, f.llm,
		session.WithWorkdirFS(newWorkdirFS(f.fs, sessID)),
		session.WithHooks(
			planning.New(f.llm, planning.Option{}),
			subagents.NewHook(f.llm, subagents.Option{
				SubAgents: []subagents.ExpertAgent{
					{
						Name:     "EXPLORER",
						Describe: EXPLORER_AGENT_DESC,
						Agent:    f.agt,
					},
				},
			}),
			summarize.NewCompactHook(f.llm, 65535),
		),
	)
	return sess, nil
}

func (f *Friday) OpenSession(ctx context.Context, sessionID string) (*session.Session, error) {
	_, err := f.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	sess := session.New(sessionID, f.llm,
		session.WithWorkdirFS(newWorkdirFS(f.fs, sessionID)),
		session.WithHooks(
			planning.New(f.llm, planning.Option{}),
			subagents.NewHook(f.llm, subagents.Option{
				SubAgents: []subagents.ExpertAgent{
					{
						Name:     "EXPLORER",
						Describe: EXPLORER_AGENT_DESC,
						Agent:    f.agt,
					},
				},
			}),
			summarize.NewCompactHook(f.llm, 65535),
		),
	)

	messages, err := f.store.GetMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	for _, msg := range messages {
		sess.History = append(sess.History, *SessionMessageToMessage(&msg))
	}

	return sess, nil
}

// Chat sends a message to the agent and returns a streaming response
func (f *Friday) Chat(ctx context.Context, sess *session.Session, message string) *api.Response {
	req := &api.Request{
		UserMessage: message,
		Session:     sess,
		Tools:       f.Tools(),
	}
	return f.agt.Chat(ctx, req)
}

// Namespace returns the namespace this Friday instance operates in
func (f *Friday) Namespace() string {
	return f.namespace
}

// GetStore returns the session store
func (f *Friday) GetStore() SessionStore {
	return f.store
}

// GetWorkdirPath returns the workdir path for a session
func (f *Friday) GetWorkdirPath(sessionID string) string {
	return path.Join("/", SessionsDirName, sessionID, WorkdirName)
}

// Tools returns all available filesystem tools
func (f *Friday) Tools() []*tools.Tool {
	return []*tools.Tool{
		f.newFileReadTool(),
		f.newFileListTool(),
		f.newFileStatTool(),

		//f.newFileWriteTool(),
		//f.newDeleteTool(),
		//f.newMkdirTool(),
		//f.newRenameTool(),

		f.newSearchTool(),
	}
}

// resolveEntry resolves a path to an entry, returns (parent, entry, error)
// Uses the Friday's default namespace since FileSystem embeds the namespace
func (f *Friday) resolveEntry(ctx context.Context, inputPath string) (*types.Entry, *types.Entry, error) {
	if inputPath == "/" {
		root, err := f.fs.Root(ctx)
		return root, root, err
	}

	entryPath, err := parsePath(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid path: %w", err)
	}

	if entryPath == "" {
		entryPath = "."
	}

	parent, entry, err := f.fs.GetEntryByPath(ctx, entryPath)
	if err != nil {
		return nil, nil, fmt.Errorf("entry not found: %w", err)
	}
	return parent, entry, nil
}
