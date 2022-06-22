package types

type Rule struct {
	Filters  RuleFilter `json:"filters"`
	Selector LabelMatch `json:"selector"`
}

type RuleFilter struct {
	Logic   RuleFilterLogic `json:"logic,omitempty"`
	Filters []RuleFilter    `json:"filters,omitempty"`

	Operation RuleOperation `json:"operation,omitempty"`
	Column    string        `json:"column"`
	Value     string        `json:"value,omitempty"`
}

type RuleOperation string

const (
	RuleOpEqual     = "equal"
	RuleOpBeginWith = "prefix"
	RuleOpPattern   = "pattern"
	RuleOpBefore    = "before"
	RuleOpAfter     = "after"
	RuleOpIn        = "in"
)

type RuleFilterLogic string

const (
	RuleLogicAll = "all"
	RuleLogicAny = "any"
)
