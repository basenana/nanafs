package friday

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	fridaytypes "github.com/basenana/friday/core/types"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
)

const (
	SessionsDirName = "sessions"
	MetaFileName    = "meta.json"
	MessageFileName = "history.jsonl"
)

type SessionMeta struct {
	ID          string    `json:"id"`
	Name        string    `json:"name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	IsCompacted bool      `json:"is_compacted"`
}

type SessionMessage struct {
	Type          string `json:"type"`
	Content       string `json:"content"`
	Reasoning     string `json:"reasoning,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	ToolArguments string `json:"tool_arguments,omitempty"`
	ToolContent   string `json:"tool_content,omitempty"`
	ToolCallID    string `json:"tool_call_id,omitempty"`
	Event         string `json:"event,omitempty"`
	EntryURI      string `json:"entry_uri,omitempty"`
	Time          string `json:"time"`
}

type SessionStore interface {
	CreateSession(ctx context.Context, name string) (*SessionMeta, error)
	GetSession(ctx context.Context, id string) (*SessionMeta, error)
	ListSessions(ctx context.Context) ([]SessionMeta, error)
	DeleteSession(ctx context.Context, id string) error
	RenameSession(ctx context.Context, id string, name string) error
	AppendMessage(ctx context.Context, sessionID string, msgs ...*SessionMessage) error
	GetMessages(ctx context.Context, sessionID string) ([]SessionMessage, error)
}

type fileSessionStore struct {
	fs        *core.FileSystem
	namespace string
	mu        sync.RWMutex
}

func NewFileSessionStore(fs *core.FileSystem, namespace string) SessionStore {
	return &fileSessionStore{
		fs:        fs,
		namespace: namespace,
	}
}

func (s *fileSessionStore) ensureFridayDir(ctx context.Context) error {
	_, _, err := s.fs.GetEntryByPath(ctx, "/.friday")
	if err == nil {
		return nil
	}

	if !errors.Is(err, types.ErrNotFound) {
		return err
	}

	_, err = s.fs.CreateEntry(ctx, "/", types.EntryAttr{
		Name: ".friday",
		Kind: types.GroupKind,
	})
	return err
}

func (s *fileSessionStore) ensureSessionsDir(ctx context.Context) error {
	if err := s.ensureFridayDir(ctx); err != nil {
		return err
	}

	_, _, err := s.fs.GetEntryByPath(ctx, "/.friday/sessions")
	if err == nil {
		return nil
	}

	if !errors.Is(err, types.ErrNotFound) {
		return err
	}

	_, err = s.fs.CreateEntry(ctx, "/.friday", types.EntryAttr{
		Name: "sessions",
		Kind: types.GroupKind,
	})
	return err
}

func (s *fileSessionStore) CreateSession(ctx context.Context, name string) (*SessionMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureSessionsDir(ctx); err != nil {
		return nil, err
	}

	meta := &SessionMeta{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	sessionDir := path.Join("/.friday/sessions", meta.ID)
	_, err := s.fs.CreateEntry(ctx, "/.friday/sessions", types.EntryAttr{
		Name: meta.ID,
		Kind: types.GroupKind,
	})
	if err != nil {
		return nil, err
	}

	_, err = s.fs.CreateEntry(ctx, sessionDir, types.EntryAttr{
		Name: WorkdirName,
		Kind: types.GroupKind,
	})
	if err != nil && !errors.Is(err, types.ErrIsExist) {
		return nil, err
	}

	metaPath := path.Join(sessionDir, MetaFileName)
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}

	_, err = s.fs.CreateEntry(ctx, sessionDir, types.EntryAttr{
		Name: MetaFileName,
		Kind: types.FileKind(MetaFileName, types.RawKind),
	})
	if err != nil {
		return nil, err
	}

	_, entry, err := s.fs.GetEntryByPath(ctx, metaPath)
	if err != nil {
		return nil, err
	}

	file, err := s.fs.Open(ctx, entry.ID, types.OpenAttr{
		Write:  true,
		Create: true,
	})
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = file.Write(metaData)
	if err != nil {
		return nil, err
	}

	_, err = s.fs.CreateEntry(ctx, sessionDir, types.EntryAttr{
		Name: MessageFileName,
		Kind: types.FileKind(MessageFileName, types.RawKind),
	})
	if err != nil {
		return nil, err
	}

	return meta, nil
}

func (s *fileSessionStore) GetSession(ctx context.Context, id string) (*SessionMeta, error) {
	metaPath := s.getMetaPath(id)

	_, entry, err := s.fs.GetEntryByPath(ctx, metaPath)
	if err != nil {
		return nil, err
	}

	file, err := s.fs.Open(ctx, entry.ID, types.OpenAttr{
		Read: true,
	})
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var meta SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

func (s *fileSessionStore) ListSessions(ctx context.Context) ([]SessionMeta, error) {
	_, parentEntry, err := s.fs.GetEntryByPath(ctx, "/.friday/sessions")
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return []SessionMeta{}, nil
		}
		return nil, err
	}

	dir, err := s.fs.OpenDir(ctx, parentEntry.ID)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	var sessions []SessionMeta
	for {
		infos, err := dir.Readdir(100)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		for _, info := range infos {
			name := info.Name()
			if !info.IsDir() {
				continue
			}

			meta, err := s.GetSession(ctx, name)
			if err != nil {
				continue
			}
			sessions = append(sessions, *meta)
		}
	}

	return sessions, nil
}

func (s *fileSessionStore) DeleteSession(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureSessionsDir(ctx); err != nil {
		return err
	}

	sessionDir := path.Join("/.friday/sessions", id)
	if err := s.fs.RmGroup(ctx, sessionDir, types.DestroyEntryAttr{
		Uid:       0,
		Gid:       0,
		Recursion: true,
	}); err != nil && !errors.Is(err, types.ErrNotFound) {
		return err
	}

	return nil
}

func (s *fileSessionStore) RenameSession(ctx context.Context, id string, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureSessionsDir(ctx); err != nil {
		return err
	}

	meta, err := s.GetSession(ctx, id)
	if err != nil {
		return err
	}

	meta.Name = name
	meta.UpdatedAt = time.Now()

	metaPath := s.getMetaPath(id)
	_, entry, err := s.fs.GetEntryByPath(ctx, metaPath)
	if err != nil {
		return err
	}

	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	file, err := s.fs.Open(ctx, entry.ID, types.OpenAttr{
		Write: true,
		Trunc: true,
	})
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(metaData)
	return err
}

func (s *fileSessionStore) AppendMessage(ctx context.Context, sessionID string, msgs ...*SessionMessage) error {
	if len(msgs) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureSessionsDir(ctx); err != nil {
		return err
	}

	var data []byte
	for _, msg := range msgs {
		msgData, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		data = append(data, msgData...)
		data = append(data, '\n')
	}

	msgPath := s.getMsgPath(sessionID)

	entry, err := s.createOrGetEntry(ctx, msgPath, MessageFileName)
	if err != nil {
		return err
	}

	file, err := s.fs.Open(ctx, entry.ID, types.OpenAttr{
		Write: true,
	})
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func (s *fileSessionStore) createOrGetEntry(ctx context.Context, msgPath, msgFileName string) (*types.Entry, error) {
	_, entry, err := s.fs.GetEntryByPath(ctx, msgPath)
	if err == nil {
		return entry, nil
	}

	if !errors.Is(err, types.ErrNotFound) {
		return nil, err
	}

	entry, err = s.fs.CreateEntry(ctx, "/.friday/sessions", types.EntryAttr{
		Name: msgFileName,
		Kind: types.FileKind(msgFileName, types.GroupKind),
	})
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (s *fileSessionStore) GetMessages(ctx context.Context, sessionID string) ([]SessionMessage, error) {
	msgPath := s.getMsgPath(sessionID)

	_, entry, err := s.fs.GetEntryByPath(ctx, msgPath)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return []SessionMessage{}, nil
		}
		return nil, err
	}

	file, err := s.fs.Open(ctx, entry.ID, types.OpenAttr{
		Read: true,
	})
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return []SessionMessage{}, nil
	}

	var messages []SessionMessage
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var msg SessionMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

func (s *fileSessionStore) getMetaPath(id string) string {
	return path.Join("/.friday/sessions", id, MetaFileName)
}

func (s *fileSessionStore) getMsgPath(id string) string {
	return path.Join("/.friday/sessions", id, MessageFileName)
}

func MessageToSessionMessage(msg *fridaytypes.Message) *SessionMessage {
	sessionMsg := &SessionMessage{
		Time: msg.Time,
	}

	if msg.UserMessage != "" {
		sessionMsg.Type = "user"
		sessionMsg.Content = msg.UserMessage
	} else if msg.AgentMessage != "" || msg.AssistantMessage != "" {
		sessionMsg.Type = "assistant"
		sessionMsg.Content = msg.AssistantMessage
		if msg.AssistantMessage == "" {
			sessionMsg.Content = msg.AgentMessage
		}
		sessionMsg.Reasoning = msg.AssistantReasoning
	} else if msg.ToolName != "" {
		sessionMsg.Type = "tool"
		sessionMsg.ToolName = msg.ToolName
		sessionMsg.ToolArguments = msg.ToolArguments
		sessionMsg.ToolContent = msg.ToolContent
		sessionMsg.ToolCallID = msg.ToolCallID
	}

	return sessionMsg
}

func SessionMessageToMessage(msg *SessionMessage) *fridaytypes.Message {
	tmsg := &fridaytypes.Message{
		Time: msg.Time,
	}

	switch msg.Type {
	case "user":
		tmsg.UserMessage = msg.Content
	case "assistant":
		tmsg.AssistantMessage = msg.Content
		tmsg.AssistantReasoning = msg.Reasoning
	case "tool":
		tmsg.ToolName = msg.ToolName
		tmsg.ToolArguments = msg.ToolArguments
		tmsg.ToolContent = msg.ToolContent
		tmsg.ToolCallID = msg.ToolCallID
	}

	return tmsg
}
