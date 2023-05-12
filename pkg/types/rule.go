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

type Rule struct {
	Filters  RuleFilter `json:"filters"`
	Selector LabelMatch `json:"selector"`
}

type RuleFilter struct {
	Logic   RuleFilterLogic `json:"logic,omitempty"`
	Filters []RuleFilter    `json:"filters,omitempty"`

	Operation RuleOperation `json:"operation,omitempty"`
	Column    string        `json:"column,omitempty"`
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
