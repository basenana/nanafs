package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type Scratchpad interface {
	ListNotes(ctx context.Context) ([]*ScratchpadNote, error)
	ReadNote(ctx context.Context, noteID string) (*ScratchpadNote, error)
	WriteNote(ctx context.Context, note *ScratchpadNote) (*ScratchpadNote, error)
}

type ScratchpadNote struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Content  string `json:"content,omitempty"`
	Filtered string `json:"filtered,omitempty"`
}

type inMemoryScratchpad struct {
	records map[string]*ScratchpadNote
	mutex   sync.RWMutex
}

func NewInMemoryScratchpad() Scratchpad {
	return &inMemoryScratchpad{
		records: make(map[string]*ScratchpadNote),
		mutex:   sync.RWMutex{},
	}
}

var _ Scratchpad = &inMemoryScratchpad{}

func (m *inMemoryScratchpad) ListNotes(ctx context.Context) ([]*ScratchpadNote, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]*ScratchpadNote, 0, len(m.records))
	for _, f := range m.records {
		result = append(result, f)
	}
	return result, nil
}

func (m *inMemoryScratchpad) ReadNote(ctx context.Context, noteID string) (*ScratchpadNote, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	f, ok := m.records[noteID]
	if !ok {
		return nil, fmt.Errorf("note does not exist")
	}
	return f, nil
}

func (m *inMemoryScratchpad) WriteNote(ctx context.Context, f *ScratchpadNote) (*ScratchpadNote, error) {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.records[f.ID] = f
	return f, nil
}

func ScratchpadReadTools(sp Scratchpad) []*Tool {
	return []*Tool{
		NewTool("list_all_from_scratchpad",
			WithDescription("Your previous work will be saved in the scratchpad. This tool can list all notes that have been saved."),
			WithToolHandler(func(ctx context.Context, request *Request) (*Result, error) {
				notes, err := sp.ListNotes(ctx)
				if err != nil {
					return NewToolResultError(err.Error()), nil
				}

				for i := range notes {
					notes[i].Content = "" // hide content to save tokens
				}

				return NewToolResultText(Res2Str(notes)), nil
			}),
		),
		NewTool("read_note_from_scratchpad",
			WithDescription("Use this tool to retrieve the full content of a note from the scratchpad."),
			WithString("note_id",
				Required(),
				Description("The id of note. If you don't know the id, you need to use `list_all_from_scratchpad` to find it."),
			),
			WithToolHandler(func(ctx context.Context, request *Request) (*Result, error) {
				noteId, ok := request.Arguments["note_id"].(string)
				if !ok || noteId == "" {
					return nil, fmt.Errorf("missing required parameter: note_id")
				}
				note, err := sp.ReadNote(ctx, noteId)
				if err != nil {
					return NewToolResultError(err.Error()), nil
				}

				return NewToolResultText(Res2Str(note)), nil
			}),
		),
		NewTool("grep_note_from_scratchpad",
			WithDescription("Use this tool to grep/filter content from a note in the scratchpad based on keywords."),
			WithString("note_id",
				Required(),
				Description("The id of note. If you don't know the id, you need to use `list_all_from_scratchpad` to find it."),
			),
			WithArray("keywords",
				Required(),
				Items(map[string]interface{}{"type": "string", "description": "Keyword to filter note content; only matching lines and their context are returned."}),
				Description("Keywords are related by \"or\". Very useful for searching in long texts."),
			),
			WithToolHandler(func(ctx context.Context, request *Request) (*Result, error) {
				noteId, ok := request.Arguments["note_id"].(string)
				if !ok || noteId == "" {
					return nil, fmt.Errorf("missing required parameter: note_id")
				}
				note, err := sp.ReadNote(ctx, noteId)
				if err != nil {
					return NewToolResultError(err.Error()), nil
				}

				keywordsRaw, ok := request.Arguments["keywords"].([]any)
				if !ok || len(keywordsRaw) == 0 {
					return nil, fmt.Errorf("missing required parameter: keywords")
				}
				var keywords []string
				for _, k := range keywordsRaw {
					keyword, ok := k.(string)
					if ok {
						keywords = append(keywords, keyword)
					}
				}

				note.Filtered = CutToSafeLength(GrepC(note.Content, 3, keywords...), 5000)
				if note.Filtered == "" {
					note.Filtered = "no filtered content"
				}
				note.Content = ""

				return NewToolResultText(Res2Str(note)), nil
			}),
		),
	}
}

func ScratchpadWriteTools(sp Scratchpad) []*Tool {
	return []*Tool{
		NewTool("create_note_from_scratchpad",
			WithDescription("create new note from the scratchpad for future access."),
			WithString("title",
				Required(),
				Description("The title of the note, convenient for subsequent quick lookup, DO NOT exceed 10 words."),
			),
			WithString("content",
				Required(),
				Description("Note content that needs to be saved"),
			),
			WithToolHandler(func(ctx context.Context, request *Request) (*Result, error) {
				title, ok := request.Arguments["title"].(string)
				if !ok || title == "" {
					return nil, fmt.Errorf("missing required parameter: title")
				}
				content, ok := request.Arguments["content"].(string)
				if !ok || content == "" {
					return nil, fmt.Errorf("missing required parameter: content")
				}

				n := &ScratchpadNote{Title: title, Content: content}
				n, err := sp.WriteNote(ctx, n)
				if err != nil {
					return NewToolResultError(err.Error()), nil
				}

				return NewToolResultText(fmt.Sprintf("note %s created", n.ID)), nil
			}),
		),
		NewTool("update_note_from_scratchpad",
			WithDescription("Update the content of note from the scratchpad for future access."),
			WithString("note_id",
				Required(),
				Description("The id of the note"),
			),
			WithString("title",
				Required(),
				Description("The title of the note, convenient for subsequent quick lookup, DO NOT exceed 10 words."),
			),
			WithString("content",
				Required(),
				Description("The content that needs to be saved"),
			),
			WithToolHandler(func(ctx context.Context, request *Request) (*Result, error) {
				noteId, ok := request.Arguments["note_id"].(string)
				if !ok || noteId == "" {
					return nil, fmt.Errorf("missing required parameter: note_id")
				}
				title, ok := request.Arguments["title"].(string)
				if !ok || title == "" {
					return nil, fmt.Errorf("missing required parameter: title")
				}
				content, ok := request.Arguments["content"].(string)
				if !ok || content == "" {
					return nil, fmt.Errorf("missing required parameter: content")
				}

				n := &ScratchpadNote{ID: noteId, Title: title, Content: content}
				n, err := sp.WriteNote(ctx, n)
				if err != nil {
					return NewToolResultError(err.Error()), nil
				}

				return NewToolResultText(fmt.Sprintf("note %s upated", n.ID)), nil
			}),
		),
	}
}
