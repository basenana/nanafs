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

import (
	"encoding/json"
	"github.com/basenana/nanafs/pkg/types"
)

func objectToMap(obj *object) map[string]interface{} {
	raw, _ := json.Marshal(obj)
	result := make(map[string]interface{})
	_ = json.Unmarshal(raw, &result)
	return result
}

func ruleLabelMatch(rule types.Rule) *types.LabelMatch {
	if rule.Labels != nil {
		return rule.Labels
	}

	var lm *types.LabelMatch
	if rule.Logic == types.RuleLogicAll {
		allLms := make([]types.LabelMatch, 0)
		for _, l := range rule.Rules {
			oneLm := ruleLabelMatch(l)
			if oneLm != nil {
				allLms = append(allLms, *oneLm)
			}
		}
		if len(allLms) > 0 {
			mergedLm := mergeLabelMatch(allLms)
			lm = &mergedLm
		}
	}

	return lm
}

func mergeLabelMatch(labelMatches []types.LabelMatch) types.LabelMatch {
	if len(labelMatches) == 0 {
		return labelMatches[1]
	}

	merged := types.LabelMatch{
		Include: make([]types.Label, 0),
		Exclude: make([]string, 0),
	}

	for _, lm := range labelMatches {
		merged.Include = append(merged.Include, lm.Include...)
		merged.Exclude = append(merged.Exclude, lm.Exclude...)
	}

	return merged
}

func mergeRules(rules []types.Rule) types.Rule {
	if len(rules) == 1 {
		return rules[0]
	}
	return types.Rule{Logic: types.RuleLogicAll, Rules: rules}
}
