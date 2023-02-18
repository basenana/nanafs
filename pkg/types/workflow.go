package types

type WorkflowSpec struct {
	Name  string             `json:"name"`
	Rule  Rule               `json:"rule,omitempty"`
	Steps []WorkflowStepSpec `json:"steps,omitempty"`
}

type WorkflowStepSpec struct {
	Name   string    `json:"name"`
	Plugin PlugScope `json:"plugin"`
}
