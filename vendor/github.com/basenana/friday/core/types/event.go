package types

type Event struct {
	Answer     *Answer           `json:"answer,omitempty"`
	Delta      *Delta            `json:"delta,omitempty"`
	Stage      *Stage            `json:"stage,omitempty"`
	Data       *EventData        `json:"data,omitempty"`
	ExtraValue map[string]string `json:"extra_value,omitempty"`
}

type Delta struct {
	Content   string `json:"content,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}

type EventData struct {
	ToolUse *ToolUse `json:"tool_use,omitempty"`
}

func NewAnsEvent(report string) *Event {
	return &Event{Answer: &Answer{Report: report}}
}

func NewContentEvent(content string) *Event {
	return &Event{Delta: &Delta{Content: content}}
}

func NewReasoningEvent(reasoning string) *Event {
	return &Event{Delta: &Delta{Reasoning: reasoning}}
}

func NewToolUseEvent(name, arg, desc, result string) *Event {
	return &Event{Data: &EventData{ToolUse: &ToolUse{Name: name, Args: arg, Describe: desc, Result: result}}}
}

func NewStageUpdateEvent(stage Stage) *Event {
	return &Event{Stage: &stage}
}

type Answer struct {
	Report string `json:"report,omitempty"`
}

type Stage struct {
	ID       string      `json:"id"`
	Status   StageStatus `json:"status"`
	Message  string      `json:"message,omitempty"`
	Describe string      `json:"describe"`
}

type StageStatus string

const (
	Submitted StageStatus = "submitted"
	Working   StageStatus = "working"
	Completed StageStatus = "completed"
	Canceled  StageStatus = "canceled"
	Failed    StageStatus = "failed"
	Unknown   StageStatus = "unknown"
)

type ToolUse struct {
	Name     string `json:"name"`
	Args     string `json:"args"`
	Describe string `json:"describe"`
	Result   string `json:"result,omitempty"`
}
