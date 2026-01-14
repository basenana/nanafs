package openai

import (
	"context"
	"encoding/json"
	"encoding/xml"

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
	History() []types.Message
	ToolDefines() []ToolDefine
}

type CompatibleChunk struct {
	Content          *string `json:"content"`
	ReasoningContent *string `json:"reasoning_content"`
}

func NewSimpleRequest(systemMessage string, history ...types.Message) Request {
	return &simpleRequest{system: systemMessage, history: history}
}

func NewToolsRequest(request Request, tools []ToolDefine) Request {

	var (
		filtered    []ToolDefine
		existedTool = make(map[string]struct{})
	)

	for _, t := range tools {
		_, exists := existedTool[t.Name]
		if exists {
			continue
		}
		filtered = append(filtered, t)
		existedTool[t.Name] = struct{}{}
	}

	tools = filtered
	req, ok := request.(*simpleRequest)
	if ok {
		req.tools = tools
		return req
	}

	var (
		system  string
		history = request.History()
	)

	if len(history) > 0 {
		system = history[0].SystemMessage
	}
	return &simpleRequest{system: system, tools: tools, history: history}
}

type ToolDefine struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolUse struct {
	XMLName   xml.Name `xml:"tool_use" json:"-"`
	ID        string   `xml:"id" json:"id"`
	Name      string   `xml:"name" json:"name"`
	Arguments string   `xml:"arguments" json:"arguments"`
	Error     string   `xml:"error" json:"error"`

	Reasoning string `xml:"-" json:"-"` // tool use in reasoning like deepseek v3.2
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
	system  string
	tools   []ToolDefine
	history []types.Message
}

func (s *simpleRequest) History() []types.Message {
	result := make([]types.Message, 0, len(s.history)+1)
	result = append(result, types.Message{SystemMessage: s.system})
	result = append(result, s.history...)
	return result
}

func (s *simpleRequest) ToolDefines() []ToolDefine {
	return s.tools
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
