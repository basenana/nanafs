package rules

import (
	"encoding/json"
	"github.com/basenana/nanafs/pkg/types"
	"regexp"
	"strings"
	"time"
)

type RuleOperation interface {
	Apply(value *types.Object) bool
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
	Content   time.Time
}

func (b Before) Apply(value *types.Object) bool {
	t, err := time.Parse("2006-01-02T15:04:05Z07:00", Get(b.ColumnKey, value).(string))
	if err != nil {
		return false
	}
	return t.Before(b.Content)
}

type After struct {
	ColumnKey string
	Content   time.Time
}

func (a After) Apply(value *types.Object) bool {
	t, err := time.Parse("2006-01-02T15:04:05Z07:00", Get(a.ColumnKey, value).(string))
	if err != nil {
		return false
	}
	return t.After(a.Content)
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
