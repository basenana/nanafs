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

package group

import (
	"encoding/json"
	"github.com/basenana/nanafs/pkg/types"
	"regexp"
	"strings"
	"time"
)

const (
	timeOpFmt     = "2006-01-02 15:04:05"
	itemDelimiter = ","
)

type RuleOperation interface {
	Apply(value *types.Object) bool
}

func NewRuleOperation(opType types.RuleOperation, col, val string) RuleOperation {
	switch opType {
	case types.RuleOpEqual:
		return Equal{ColumnKey: col, Content: val}
	case types.RuleOpBeginWith:
		return BeginWith{ColumnKey: col, Content: val}
	case types.RuleOpPattern:
		return Pattern{ColumnKey: col, Content: val}
	case types.RuleOpBefore, types.RuleOpAfter:
		t, err := time.Parse(timeOpFmt, val)
		if err != nil {
			if opType == types.RuleOpBefore {
				return Before{ColumnKey: col, Time: nil}
			}
			if opType == types.RuleOpAfter {
				return After{ColumnKey: col, Time: nil}
			}
		}
		if opType == types.RuleOpBefore {
			return Before{ColumnKey: col, Time: &t}
		}
		if opType == types.RuleOpAfter {
			return After{ColumnKey: col, Time: &t}
		}
	case types.RuleOpIn:
		return In{
			ColumnKey: col,
			Content:   strings.Split(val, itemDelimiter),
		}
	}
	return nil
}

type Equal struct {
	ColumnKey string
	Content   string
}

func (m Equal) Apply(value *types.Object) bool {
	return Get(m.ColumnKey, value).(string) == m.Content
}

type Pattern struct {
	ColumnKey string
	Content   string
}

func (p Pattern) Apply(value *types.Object) bool {
	matched, _ := regexp.Match(p.Content, []byte(Get(p.ColumnKey, value).(string)))
	return matched
}

type BeginWith struct {
	ColumnKey string
	Content   string
}

func (b BeginWith) Apply(value *types.Object) bool {
	return strings.HasPrefix(Get(b.ColumnKey, value).(string), b.Content)
}

type Before struct {
	ColumnKey string
	Time      *time.Time
}

func (b Before) Apply(value *types.Object) bool {
	if b.Time == nil {
		return false
	}
	t, err := time.Parse("2006-01-02T15:04:05Z07:00", Get(b.ColumnKey, value).(string))
	if err != nil {
		return false
	}
	return t.Before(*b.Time)
}

type After struct {
	ColumnKey string
	Time      *time.Time
}

func (a After) Apply(value *types.Object) bool {
	if a.Time == nil {
		return false
	}
	t, err := time.Parse("2006-01-02T15:04:05Z07:00", Get(a.ColumnKey, value).(string))
	if err != nil {
		return false
	}
	return t.After(*a.Time)
}

type In struct {
	ColumnKey string
	Content   []string
}

func (i In) Apply(value *types.Object) bool {
	v := Get(i.ColumnKey, value).(string)
	for _, c := range i.Content {
		if v == c {
			return true
		}
	}
	return false
}

func Get(columnKey string, o *types.Object) interface{} {
	res, err := json.Marshal(o)
	if err != nil {
		return nil
	}

	return getVal(columnKey, res)
}

func getVal(key string, data []byte) interface{} {
	resMap := make(map[string]interface{})
	err := json.Unmarshal(data, &resMap)
	if err != nil {
		return nil
	}

	keyParts := strings.SplitN(key, ".", 1)
	subKey := keyParts[0]
	if len(keyParts) == 1 {
		return resMap[subKey]
	}

	res, err := json.Marshal(resMap[subKey])
	if err != nil {
		return nil
	}
	return getVal(keyParts[1], res)
}
