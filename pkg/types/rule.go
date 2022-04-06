package types

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

type Rule struct {
	Logic     string
	Rules     []Rule
	Operation Operation
}

func (r Rule) Apply(value *Object) bool {
	if len(r.Rules) == 0 {
		return r.Operation.Apply(value)
	}
	if r.Logic == "all" {
		for _, rule := range r.Rules {
			if !rule.Apply(value) {
				return false
			}
		}
		return true
	}
	if r.Logic == "any" {
		for _, rule := range r.Rules {
			if rule.Apply(value) {
				return true
			}
		}
		return false
	}
	return true
}

type Operation interface {
	Apply(value *Object) bool
}

type Equal struct {
	ColumnKey string
	Content   string
}

func (m Equal) Apply(value *Object) bool {
	return Get(m.ColumnKey, value).(string) == m.Content
}

type Pattern struct {
	ColumnKey string
	Content   string
}

func (p Pattern) Apply(value *Object) bool {
	matched, _ := regexp.Match(p.Content, []byte(Get(p.ColumnKey, value).(string)))
	return matched
}

type BeginWith struct {
	ColumnKey string
	Content   string
}

func (b BeginWith) Apply(value *Object) bool {
	return strings.HasPrefix(Get(b.ColumnKey, value).(string), b.Content)
}

type Before struct {
	ColumnKey string
	Content   time.Time
}

func (b Before) Apply(value *Object) bool {
	return Get(b.ColumnKey, value).(time.Time).Before(b.Content)
}

type After struct {
	ColumnKey string
	Content   time.Time
}

func (a After) Apply(value *Object) bool {
	return Get(a.ColumnKey, value).(time.Time).After(a.Content)
}

type In struct {
	ColumnKey string
	Content   []string
}

func (i In) Apply(value *Object) bool {
	v := Get(i.ColumnKey, value).(string)
	for _, c := range i.Content {
		if v == c {
			return true
		}
	}
	return false
}

func Get(columnKey string, o *Object) interface{} {
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
