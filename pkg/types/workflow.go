package types

type WorkflowSpec struct {
	Id    string             `json:"id"`
	Name  string             `json:"name"`
	Rule  Rule               `json:"rule,omitempty"`
	Steps []WorkflowStepSpec `json:"steps,omitempty"`
}

type WorkflowStepSpec struct {
	Name   string    `json:"name"`
	Plugin PlugScope `json:"plugin"`
}

type WorkflowJob struct {
	Id       string            `json:"id"`
	Workflow string            `json:"workflow"`
	Status   string            `json:"status"`
	Message  string            `json:"message"`
	Steps    []WorkflowJobStep `json:"steps"`
}

type WorkflowJobStep struct {
	StepName string    `json:"step_name"`
	Message  string    `json:"message"`
	Status   string    `json:"status"`
	Plugin   PlugScope `json:"plugin"`
}
