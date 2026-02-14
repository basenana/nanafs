package types

type Event struct {
	Delta      *Delta            `json:"delta,omitempty"`
	ExtraValue map[string]string `json:"extra_value,omitempty"`
}

type Delta struct {
	Content   string `json:"content,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}

func NewContentEvent(content string) *Event {
	return &Event{Delta: &Delta{Content: content}}
}

func NewReasoningEvent(reasoning string) *Event {
	return &Event{Delta: &Delta{Reasoning: reasoning}}
}
