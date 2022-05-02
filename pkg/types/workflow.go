package types

import "time"

type Workflow struct {
	Name    string   `json:"name"`
	Rule    WFRule   `json:"rule"`
	Actions []string `json:"actions"`
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
	Logic     string    `json:"logic"`
	Rules     []WFRule  `json:"rules"`
	Equal     *CommOpe  `json:"equal"`
	BeginWith *CommOpe  `json:"begin_with"`
	Pattern   *CommOpe  `json:"pattern"`
	Before    *TimeOpe  `json:"before"`
	After     *TimeOpe  `json:"after"`
	In        *MutilOpe `json:"in"`
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
	ColumnKey string
	Content   []string
}

type TimeOpe struct {
	ColumnKey string
	Content   time.Time
}
