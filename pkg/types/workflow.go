package types

import "time"

type Workflow struct {
	Name    string   `json:"name"`
	Rule    WFRule   `json:"rule,omitempty"`
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

type WFRule struct {
	Logic     string    `json:"logic,omitempty"`
	Rules     []WFRule  `json:"rules,omitempty"`
	Equal     *CommOpe  `json:"equal,omitempty"`
	BeginWith *CommOpe  `json:"begin_with,omitempty"`
	Pattern   *CommOpe  `json:"pattern,omitempty"`
	Before    *TimeOpe  `json:"before,omitempty"`
	After     *TimeOpe  `json:"after,omitempty"`
	In        *MutilOpe `json:"in,omitempty"`
}

func (w WFRule) ToRule() Rule {
	rules := make([]Rule, len(w.Rules))
	for i, r := range w.Rules {
		rules[i] = r.ToRule()
	}
	var ope Operation
	if w.Equal != nil {
		ope = Equal{w.Equal.ColumnKey, w.Equal.Content}
	}
	if w.BeginWith != nil {
		ope = BeginWith{w.BeginWith.ColumnKey, w.BeginWith.Content}
	}
	if w.Pattern != nil {
		ope = Pattern{w.Pattern.ColumnKey, w.Pattern.Content}
	}
	if w.Before != nil {
		ope = Before{w.Before.ColumnKey, w.Before.Content}
	}
	if w.After != nil {
		ope = After{w.After.ColumnKey, w.After.Content}
	}
	if w.In != nil {
		ope = In{w.In.ColumnKey, w.In.Content}
	}
	return Rule{
		Logic:     w.Logic,
		Rules:     rules,
		Operation: ope,
	}
}

type CommOpe struct {
	ColumnKey string `json:"column_key"`
	Content   string `json:"content"`
}

type MutilOpe struct {
	ColumnKey string   `json:"column_key"`
	Content   []string `json:"content"`
}

type TimeOpe struct {
	ColumnKey string    `json:"column_key"`
	Content   time.Time `json:"content"`
}
