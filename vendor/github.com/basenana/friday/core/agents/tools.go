package agents

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/session"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/friday/core/types"
)

var (
	buildInTools []openai.ToolDefine
)

type ToolUse struct {
	XMLName   xml.Name `xml:"tool_use"`
	GenID     string   `xml:"id"`
	Name      string   `xml:"name"`
	Arguments string   `xml:"arguments"`
}

func (t *ToolUse) ID() string {
	if t.GenID != "" {
		return t.GenID
	}
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(t.Name))
	_, _ = hasher.Write([]byte(t.Arguments))
	hashValue := hasher.Sum64()
	t.GenID = fmt.Sprintf("call-%s-%d", t.Name, hashValue)
	return t.GenID
}

func toolCall(ctx context.Context, sess *session.Session, use *ToolUse, td *tools.Tool) (string, error) {
	req := &tools.Request{Arguments: make(map[string]interface{}), SessionID: sess.ID}
	if err := json.Unmarshal([]byte(use.Arguments), &req.Arguments); err != nil {
		return "", fmt.Errorf("unmarshal json argument failed: %s", err)
	}

	session.SendEvent(sess.Root.ID, NewToolUseEvent("react", use))
	result, err := td.Handler(ctx, req)
	if err != nil {
		session.SendEvent(sess.Root.ID, NewToolUseResultEvent("react", use, err.Error()))
		return "", err
	}

	content, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal tool %s result failed: %s", use.Name, err)
	}

	session.SendEvent(sess.Root.ID, NewToolUseResultEvent("react", use, string(content)))
	return string(content), nil
}

func newLLMRequest(systemMessage string, sess *session.Session, toolList []*tools.Tool) openai.Request {
	var toolDef []openai.ToolDefine
	for _, t := range buildInTools {
		toolDef = append(toolDef, t)
	}

	for _, t := range toolList {
		toolDef = append(toolDef, t)
	}

	req := openai.NewSimpleRequest(systemMessage, sess.History...)
	req.SetToolDefines(toolDef)
	return req
}

func NewToolUseEvent(source string, use *ToolUse) *types.Event {
	data, _ := json.Marshal(use)
	return &types.Event{
		Id:              types.NewID(),
		Type:            "tool_use",
		Source:          source,
		SpecVersion:     "1.0",
		DataContentType: "application/json",
		Data:            string(data),
		Time:            time.Now(),
	}
}

func NewToolUseResultEvent(source string, use *ToolUse, result string) *types.Event {
	data, _ := json.Marshal(map[string]interface{}{
		"id":     use.ID(),
		"result": result,
	})

	return &types.Event{
		Id:              types.NewID(),
		Type:            "tool_use_result",
		Source:          source,
		SpecVersion:     "1.0",
		DataContentType: "application/json",
		Data:            string(data),
		Time:            time.Now(),
	}
}
