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

package types

const (
	RuleLogicAll = "all"
	RuleLogicAny = "any"
	RuleLogicNot = "not"

	RuleOpEqual     = "equal"
	RuleOpBeginWith = "prefix"
	RuleOpEndWith   = "suffix"
	RuleOpPattern   = "pattern"
	RuleOpBefore    = "before"
	RuleOpAfter     = "after"
	RuleOpIn        = "in"
)

type Rule struct {
	Logic string `json:"logic,omitempty"`
	Rules []Rule `json:"rules,omitempty"`

	Operation string `json:"operation,omitempty"`
	Column    string `json:"column,omitempty"`
	Value     string `json:"value,omitempty"`

	Labels *LabelMatch `json:"labels,omitempty"`
}
