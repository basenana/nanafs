package group

import "github.com/basenana/nanafs/pkg/types"

func objectFilter(filter types.RuleFilter, value *types.Object) bool {
	if filter.Operation != "" {
		op := NewRuleOperation(filter.Operation, filter.Column, filter.Value)
		return op.Apply(value)
	}

	switch filter.Logic {
	case types.RuleLogicAll:
		for _, f := range filter.Filters {
			if !objectFilter(f, value) {
				return false
			}
		}
		return true
	case types.RuleLogicAny:
		for _, f := range filter.Filters {
			if objectFilter(f, value) {
				return true
			}
		}
		return false

	}
	return false
}

func RuleMatch(rule *types.Rule, value *types.Object) bool {
	return objectFilter(rule.Filters, value)
}

func validateRuleSpec(rule *types.Rule) error {
	return nil
}
