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
	"github.com/basenana/nanafs/pkg/types"
	"regexp"
	"strings"
	"time"
)

const (
	timeOpFmt     = "2006-01-02 15:04:05"
	itemDelimiter = ","
)

type Operation interface {
	Apply(value map[string]interface{}) bool
}

func NewRuleOperation(opType, col, val string) Operation {
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
			return AlwaysFalse{}
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

func (m Equal) Apply(value map[string]interface{}) bool {
	return Get(m.ColumnKey, value) == m.Content
}

type Pattern struct {
	ColumnKey string
	Content   string
}

func (p Pattern) Apply(value map[string]interface{}) bool {
	matched, _ := regexp.Match(p.Content, []byte(Get(p.ColumnKey, value)))
	return matched
}

type BeginWith struct {
	ColumnKey string
	Content   string
}

func (b BeginWith) Apply(value map[string]interface{}) bool {
	return strings.HasPrefix(Get(b.ColumnKey, value), b.Content)
}

type Before struct {
	ColumnKey string
	Time      *time.Time
}

func (b Before) Apply(value map[string]interface{}) bool {
	if b.Time == nil {
		return false
	}
	t, err := time.Parse(time.RFC3339, Get(b.ColumnKey, value))
	if err != nil {
		return false
	}
	return t.Before(*b.Time)
}

type After struct {
	ColumnKey string
	Time      *time.Time
}

func (a After) Apply(value map[string]interface{}) bool {
	if a.Time == nil {
		return false
	}
	t, err := time.Parse(time.RFC3339, Get(a.ColumnKey, value))
	if err != nil {
		return false
	}
	return t.After(*a.Time)
}

type In struct {
	ColumnKey string
	Content   []string
}

func (i In) Apply(value map[string]interface{}) bool {
	v := Get(i.ColumnKey, value)
	for _, c := range i.Content {
		if v == c {
			return true
		}
	}
	return false
}

type AlwaysFalse struct{}

func (a AlwaysFalse) Apply(value map[string]interface{}) bool {
	return false
}

func Get(columnKey string, o map[string]interface{}) string {
	result, ok := getVal(columnKey, o).(string)
	if !ok {
		return ""
	}
	return result
}

func getVal(key string, resMap map[string]interface{}) interface{} {
	keyParts := strings.SplitN(key, ".", 2)
	subKey := keyParts[0]
	if len(keyParts) == 1 {
		return resMap[subKey]
	}

	nextMap := resMap[subKey]
	if nextMap == nil {
		return ""
	}

	return getVal(keyParts[1], nextMap.(map[string]interface{}))
}

func labelOperation(labels *types.Labels, match *types.LabelMatch) bool {
	if labels == nil {
		labels = &types.Labels{}
	}

	if match == nil {
		return false
	}

	needInclude := map[string]string{}
	needExclude := map[string]struct{}{}
	for _, in := range match.Include {
		needInclude[in.Key] = in.Value
	}
	for _, exKey := range match.Exclude {
		needExclude[exKey] = struct{}{}
	}

	for _, l := range labels.Labels {
		inVal, has := needInclude[l.Key]
		if has && inVal == l.Value {
			delete(needInclude, l.Key)
		}
		if _, hasEx := needExclude[l.Key]; hasEx {
			return false
		}
	}
	return len(needInclude) == 0
}
