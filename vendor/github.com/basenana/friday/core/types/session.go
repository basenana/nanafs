package types

import (
	"github.com/google/uuid"
)

type SessionType string

const (
	SessionHookBeforeAgent = "before_agent"
	SessionHookBeforeModel = "before_model"
	SessionHookAfterModel  = "after_model"
)

type Message struct {
	SystemMessage      string `json:"system_message,omitempty"`
	UserMessage        string `json:"user_message,omitempty"`
	AgentMessage       string `json:"agent_message,omitempty"`
	AssistantMessage   string `json:"assistant_message,omitempty"`
	AssistantReasoning string `json:"assistant_reasoning,omitempty"`

	ImageURL string `json:"image_url,omitempty"`

	ToolCallID    string `json:"tool_call_id,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	ToolArguments string `json:"tool_arguments,omitempty"`
	ToolContent   string `json:"tool_content,omitempty"`

	Metadata map[string]string `json:"-"`
	Time     string            `json:"time,omitempty"`
}

func (m Message) FuzzyTokens() int64 {
	counter := []int{
		len([]rune(m.SystemMessage)),
		len([]rune(m.UserMessage)),
		len([]rune(m.AgentMessage)),
		len([]rune(m.AssistantMessage)),
		len([]rune(m.ImageURL)),
		len([]rune(m.ToolCallID)),
		len([]rune(m.ToolName)),
		len([]rune(m.ToolArguments)),
		len([]rune(m.ToolContent)),
	}

	var total float64
	for _, c := range counter {
		total += float64(c)
	}

	return int64(total * 0.6)
}

func NewID() string {
	return uuid.New().String()
}
