/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package rule

import "github.com/basenana/nanafs/pkg/types"

func Filter(filter types.Rule, entry *types.Metadata, ex *types.ExtendData, label *types.Labels) bool {
	return entryColumnFilter(filter, objectToMap(&object{Metadata: entry, ExtendData: ex, Labels: label}), label)
}

func entryColumnFilter(filter types.Rule, obj map[string]interface{}, labels *types.Labels) bool {
	if filter.Operation != "" {
		op := NewRuleOperation(filter.Operation, filter.Column, filter.Value)
		return op.Apply(obj)
	}

	if filter.Labels != nil {
		// FIXME: support label filter
		if labels == nil {
			return true
		}
		return labelOperation(labels, filter.Labels)
	}

	switch filter.Logic {
	case types.RuleLogicAll:
		for _, f := range filter.Rules {
			if !entryColumnFilter(f, obj, labels) {
				return false
			}
		}
		return true
	case types.RuleLogicAny:
		for _, f := range filter.Rules {
			if entryColumnFilter(f, obj, labels) {
				return true
			}
		}
		return false
	case types.RuleLogicNot:
		for _, f := range filter.Rules {
			if entryColumnFilter(f, obj, labels) {
				return false
			}
		}
		return true
	}
	return false
}

type object struct {
	*types.Metadata
	*types.ExtendData
	*types.Labels
}
