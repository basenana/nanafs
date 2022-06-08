package types

type Workflow struct {
	Name    string   `json:"name"`
	Rule    Rule     `json:"rule,omitempty"`
	Actions []string `json:"actions,omitempty"`
}

type Job struct {
	Id           string `json:"id"`
	WorkflowName string `json:"workflow_name"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	Tasks        []Task `json:"tasks"`
}

type Task struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Status  string `json:"status"`
}
