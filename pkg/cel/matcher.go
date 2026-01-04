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

package cel

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/basenana/nanafs/pkg/types"
	goCEL "github.com/google/cel-go/cel"
	celTypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

func EntryMatch(ctx context.Context, entry *types.Entry, match *types.WorkflowEntryMatch) (bool, error) {
	if match.FileNamePattern != "" {
		matched, err := filepath.Match(match.FileNamePattern, entry.Name)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}

	if match.FileTypes != "" {
		ext := strings.TrimPrefix(filepath.Ext(entry.Name), ".")
		found := false
		for _, t := range strings.Split(match.FileTypes, ",") {
			if strings.TrimSpace(t) == ext {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}

	if match.MinFileSize > 0 && entry.Size < int64(match.MinFileSize) {
		return false, nil
	}
	if match.MaxFileSize > 0 && entry.Size > int64(match.MaxFileSize) {
		return false, nil
	}

	if match.CELPattern != "" {
		matched, err := EvalCEL(ctx, entry, match.CELPattern)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}

	return true, nil
}

func EvalCEL(ctx context.Context, entry *types.Entry, pattern string) (bool, error) {
	envOpts := []goCEL.EnvOption{
		goCEL.Variable("id", goCEL.IntType),
		goCEL.Variable("kind", goCEL.StringType),
		goCEL.Variable("is_group", goCEL.BoolType),
		goCEL.Variable("size", goCEL.IntType),
		goCEL.Variable("name", goCEL.StringType),
		goCEL.Variable("created_at", goCEL.IntType),
		goCEL.Variable("changed_at", goCEL.IntType),
		goCEL.Variable("modified_at", goCEL.IntType),
		goCEL.Variable("access_at", goCEL.IntType),
	}

	envOpts = append(envOpts, goCEL.Function("now",
		goCEL.Overload("now", []*goCEL.Type{}, goCEL.IntType,
			goCEL.FunctionBinding(func(args ...ref.Val) ref.Val {
				return celTypes.Int(time.Now().Unix())
			}),
		),
	))

	e, err := goCEL.NewEnv(envOpts...)
	if err != nil {
		return false, err
	}

	ast, issues := e.Compile(pattern)
	if issues != nil {
		return false, issues.Err()
	}

	vars := map[string]any{
		"id":          entry.ID,
		"kind":        string(entry.Kind),
		"is_group":    entry.IsGroup,
		"size":        entry.Size,
		"name":        entry.Name,
		"created_at":  entry.CreatedAt.Unix(),
		"changed_at":  entry.ChangedAt.Unix(),
		"modified_at": entry.ModifiedAt.Unix(),
		"access_at":   entry.AccessAt.Unix(),
	}

	prg, err := e.Program(ast)
	if err != nil {
		return false, err
	}

	act, err := goCEL.NewActivation(vars)
	if err != nil {
		return false, err
	}

	out, _, err := prg.Eval(act)
	if err != nil {
		return false, err
	}

	return out == celTypes.Bool(true), nil
}

func BuildCELFilterFromMatch(match *types.WorkflowEntryMatch) string {
	var conditions []string

	if match.FileNamePattern != "" {
		sqlPattern := globToSQLLike(match.FileNamePattern)
		conditions = append(conditions, sqlPattern)
	}

	if match.FileTypes != "" {
		return ""
	}

	if match.MinFileSize > 0 {
		conditions = append(conditions, "size >= "+itoa(match.MinFileSize))
	}
	if match.MaxFileSize > 0 {
		conditions = append(conditions, "size <= "+itoa(match.MaxFileSize))
	}

	if match.ParentID > 0 {
		return ""
	}

	if match.CELPattern != "" {
		conditions = append(conditions, match.CELPattern)
	}

	return strings.Join(conditions, " && ")
}

func globToSQLLike(pattern string) string {
	re := regexp.MustCompile(`\*`)
	sqlPattern := re.ReplaceAllString(pattern, "%")
	re = regexp.MustCompile(`\?`)
	sqlPattern = re.ReplaceAllString(sqlPattern, "_")
	return "name LIKE '" + sqlPattern + "'"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	result := ""
	for i > 0 {
		result = string(rune('0'+i%10)) + result
		i /= 10
	}
	return result
}
