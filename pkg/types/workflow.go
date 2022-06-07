package types

import (
	"github.com/basenana/nanafs/utils/rules"
	"time"
)

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

func (w WFRule) ToRule() RuleCondition {
	ruleList := make([]RuleCondition, len(w.Rules))
	for i, r := range w.Rules {
		ruleList[i] = r.ToRule()
	}
	var ope rules.RuleOperation
	if w.Equal != nil {
		ope = rules.Equal{ColumnKey: w.Equal.ColumnKey, Content: w.Equal.Content}
	}
	if w.BeginWith != nil {
		ope = rules.BeginWith{ColumnKey: w.BeginWith.ColumnKey, Content: w.BeginWith.Content}
	}
	if w.Pattern != nil {
		ope = rules.Pattern{ColumnKey: w.Pattern.ColumnKey, Content: w.Pattern.Content}
	}
	if w.Before != nil {
		ope = rules.Before{ColumnKey: w.Before.ColumnKey, Content: w.Before.Content}
	}
	if w.After != nil {
		ope = rules.After{ColumnKey: w.After.ColumnKey, Content: w.After.Content}
	}
	if w.In != nil {
		ope = rules.In{ColumnKey: w.In.ColumnKey, Content: w.In.Content}
	}
	return RuleCondition{
		Logic:     w.Logic,
		Rules:     ruleList,
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
