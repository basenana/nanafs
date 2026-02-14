package openai

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"strings"

	"github.com/basenana/friday/core/types"
	"github.com/openai/openai-go"
)

type Client interface {
	Completion(ctx context.Context, request Request) Response
	CompletionNonStreaming(ctx context.Context, request Request) (string, error)
	StructuredPredict(ctx context.Context, request Request, model any) error
}

type Response interface {
	Message() <-chan Delta
	Error() <-chan error
	Tokens() Tokens
}

type Tokens struct {
	CompletionTokens int64
	PromptTokens     int64
	TotalTokens      int64
}

type Delta struct {
	Content   string    `json:"content"`
	Reasoning string    `json:"reasoning"`
	ToolUse   []ToolUse `json:"tool_use"`
}

type Request interface {
	Messages() []types.Message

	History() []types.Message
	ToolDefines() []ToolDefine
	SystemPrompt() string

	SetHistory([]types.Message)
	SetToolDefines([]ToolDefine)
	SetSystemPrompt(string)
	AppendHistory(...types.Message)
	AppendToolDefines(...ToolDefine)
	AppendSystemPrompt(...string)
}

type CompatibleChunk struct {
	Content          *string `json:"content"`
	ReasoningContent *string `json:"reasoning_content"`
}

func NewSimpleRequest(systemMessage string, history ...types.Message) Request {
	return &simpleRequest{systemPrompts: []string{systemMessage}, history: history}
}

type ToolDefine interface {
	GetName() string
	GetDescription() string
	GetParameters() map[string]any
}

type simpleToolDefine struct {
	name        string
	description string
	parameters  map[string]any
}

func (s simpleToolDefine) GetName() string               { return s.name }
func (s simpleToolDefine) GetDescription() string        { return s.description }
func (s simpleToolDefine) GetParameters() map[string]any { return s.parameters }

func NewToolDefine(name, description string, parameters map[string]any) ToolDefine {
	return simpleToolDefine{name: name, description: description, parameters: parameters}
}

type ToolUse struct {
	XMLName   xml.Name `xml:"tool_use" json:"-"`
	ID        string   `xml:"id" json:"id"`
	Name      string   `xml:"name" json:"name"`
	Arguments string   `xml:"arguments" json:"arguments"`
	Error     string   `xml:"error" json:"error"`

	Reasoning string `xml:"-" json:"-"` // tool use in reasoning like deepseek v3.2
}

type Apply struct {
	ToolUse []ToolUse

	Continue bool
	Abort    bool
}

type Reasoning struct {
	XMLName xml.Name `xml:"think"`
	Content string   `xml:",chardata"`
}

type Model struct {
	Name             string
	Temperature      *float64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	StrictMode       bool
	QPM              int64
	Proxy            string
}

type compatibleResponse struct {
	stream chan Delta
	buf    *xmlParser
	err    chan error
	token  Tokens
}

func (r *compatibleResponse) nextChoice(chunk openai.ChatCompletionChunkChoice) {
	delta := &CompatibleChunk{}
	err := json.Unmarshal([]byte(chunk.Delta.RawJSON()), delta)
	if err != nil {
		goto FailBack
	}

	switch {
	case delta.Content != nil && len(*delta.Content) > 0:
		content := *delta.Content
		msgList := r.buf.write(content)
		for _, msg := range msgList {
			r.stream <- msg
		}

	case delta.ReasoningContent != nil && len(*delta.ReasoningContent) > 0:
		r.stream <- Delta{Reasoning: *delta.ReasoningContent}

	default:
		goto FailBack

	}
	return

FailBack:
	switch {

	case len(chunk.Delta.Content) > 0:
		msgList := r.buf.write(chunk.Delta.Content)
		for _, msg := range msgList {
			r.stream <- msg
		}

		//case len(chunk.Delta.JSON.ExtraFields) > 0:
		//	if f, ok := chunk.Delta.JSON.ExtraFields["reasoning_content"]; ok {
		//		r.stream <- Delta{Reasoning: f.Raw()}
		//	}
	}
}

func (r *compatibleResponse) updateUsage(chunk openai.CompletionUsage) {
	r.token.CompletionTokens += chunk.CompletionTokens
	r.token.PromptTokens += chunk.PromptTokens
	r.token.TotalTokens += chunk.TotalTokens
}

func (r *compatibleResponse) fail(err error) {
	r.err <- err
}

func (r *compatibleResponse) Message() <-chan Delta {
	return r.stream
}

func (r *compatibleResponse) Error() <-chan error {
	return r.err
}

func (r *compatibleResponse) Tokens() Tokens {
	return r.token
}

func (r *compatibleResponse) close() {
	msgList := r.buf.flush()
	for _, msg := range msgList {
		r.stream <- msg
	}
	close(r.stream)
	close(r.err)
}

func newCompatibleResponse() *compatibleResponse {
	return &compatibleResponse{stream: make(chan Delta, 5), buf: newXmlParser(), err: make(chan error, 1)}
}

type simpleRequest struct {
	systemPrompts []string
	tools         []ToolDefine
	history       []types.Message
}

func (s *simpleRequest) Messages() []types.Message {
	result := make([]types.Message, 0, len(s.history)+1)
	result = append(result, types.Message{SystemMessage: s.SystemPrompt()})
	result = append(result, s.history...)
	return result
}

func (s *simpleRequest) History() []types.Message {
	return s.history
}

func (s *simpleRequest) ToolDefines() []ToolDefine {
	return s.tools
}

func (s *simpleRequest) SystemPrompt() string {
	return strings.Join(s.systemPrompts, "\n\n")
}

func (s *simpleRequest) SetHistory(history []types.Message) {
	s.history = history
}

func (s *simpleRequest) SetToolDefines(tools []ToolDefine) {
	var (
		filtered    []ToolDefine
		existedTool = make(map[string]struct{})
	)

	for _, t := range tools {
		_, exists := existedTool[t.GetName()]
		if exists {
			continue
		}
		filtered = append(filtered, t)
		existedTool[t.GetName()] = struct{}{}
	}
	s.tools = filtered
}

func (s *simpleRequest) SetSystemPrompt(prompt string) {
	s.systemPrompts = []string{prompt}
}

func (s *simpleRequest) AppendHistory(messages ...types.Message) {
	s.history = append(s.history, messages...)
}

func (s *simpleRequest) AppendToolDefines(tools ...ToolDefine) {
	for i, t := range tools {
		var exists bool
		for _, existing := range s.tools {
			if existing.GetName() == t.GetName() {
				exists = true
				s.tools[i] = t // override
				break
			}
		}
		if !exists {
			s.tools = append(s.tools, t)
		}
	}
}

func (s *simpleRequest) AppendSystemPrompt(prompts ...string) {
	s.systemPrompts = append(s.systemPrompts, prompts...)
}

type simpleResponse struct {
	stream chan Delta
	err    chan error

	incompleteTool *ToolUse

	token Tokens
}

func (r *simpleResponse) nextChoice(chunk openai.ChatCompletionChunkChoice) {
	cchunk := &CompatibleChunk{}
	err := json.Unmarshal([]byte(chunk.Delta.RawJSON()), cchunk)
	if err != nil {
		goto FailBack
	}

	switch {
	case cchunk.Content != nil && len(*cchunk.Content) > 0:
		r.stream <- Delta{Content: *cchunk.Content}

	case cchunk.ReasoningContent != nil && len(*cchunk.ReasoningContent) > 0:
		r.stream <- Delta{Reasoning: *cchunk.ReasoningContent}

	default:
		goto FailBack

	}
	return

FailBack:
	delta := chunk.Delta
	switch {
	case delta.Content != "":
		r.stream <- Delta{Content: delta.Content}

	case len(delta.ToolCalls) > 0:

		for _, tc := range delta.ToolCalls {
			if tc.ID != "" && (r.incompleteTool == nil || r.incompleteTool.ID != tc.ID) {
				if r.incompleteTool != nil {
					r.flushToolUse()
				}

				// new tool
				r.incompleteTool = &ToolUse{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
			}

			if r.incompleteTool == nil {
				return
			}

			r.incompleteTool.Arguments += tc.Function.Arguments
		}
	}
}

func (r *simpleResponse) flushToolUse() {
	if r.incompleteTool == nil {
		return
	}
	tu := *r.incompleteTool
	r.incompleteTool = nil

	if tu.Arguments != "" {
		data := make(map[string]any)
		if err := json.Unmarshal([]byte(tu.Arguments), &data); err != nil {
			tu.Error = err.Error()
		}
	} else {
		tu.Arguments = "{}"
	}

	r.stream <- Delta{ToolUse: []ToolUse{tu}}
}

func (r *simpleResponse) updateUsage(chunk openai.CompletionUsage) {
	r.token.CompletionTokens += chunk.CompletionTokens
	r.token.PromptTokens += chunk.PromptTokens
	r.token.TotalTokens += chunk.TotalTokens
}

func (r *simpleResponse) fail(err error) {
	r.err <- err
}

func (r *simpleResponse) Message() <-chan Delta {
	return r.stream
}

func (r *simpleResponse) Error() <-chan error {
	return r.err
}

func (r *simpleResponse) Tokens() Tokens {
	return r.token
}

func (r *simpleResponse) close() {
	r.flushToolUse()

	close(r.stream)
	close(r.err)
}

func newSimpleResponse() *simpleResponse {
	return &simpleResponse{stream: make(chan Delta, 5), err: make(chan error, 1)}
}
