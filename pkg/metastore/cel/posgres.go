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
	"fmt"
	"github.com/basenana/nanafs/pkg/cel"
	"github.com/pkg/errors"
	exprv1 "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"slices"
	"strings"
)

func convertWithParameterIndexWithPG(ctx *ConvertContext, expr *exprv1.Expr, paramIndex int) (int, error) {
	if v, ok := expr.ExprKind.(*exprv1.Expr_CallExpr); ok {
		switch v.CallExpr.Function {
		case "_||_", "_&&_":
			if len(v.CallExpr.Args) != 2 {
				return paramIndex, fmt.Errorf("invalid number of arguments for %s", v.CallExpr.Function)
			}
			if _, err := ctx.Buffer.WriteString("("); err != nil {
				return paramIndex, err
			}
			newParamIndex, err := convertWithParameterIndexWithPG(ctx, v.CallExpr.Args[0], paramIndex)
			if err != nil {
				return paramIndex, err
			}
			operator := "AND"
			if v.CallExpr.Function == "_||_" {
				operator = "OR"
			}
			if _, err := ctx.Buffer.WriteString(fmt.Sprintf(" %s ", operator)); err != nil {
				return paramIndex, err
			}
			newParamIndex, err = convertWithParameterIndexWithPG(ctx, v.CallExpr.Args[1], newParamIndex)
			if err != nil {
				return paramIndex, err
			}
			if _, err := ctx.Buffer.WriteString(")"); err != nil {
				return paramIndex, err
			}
			return newParamIndex, nil
		case "!_":
			if len(v.CallExpr.Args) != 1 {
				return paramIndex, fmt.Errorf("invalid number of arguments for %s", v.CallExpr.Function)
			}
			if _, err := ctx.Buffer.WriteString("NOT ("); err != nil {
				return paramIndex, err
			}
			newParamIndex, err := convertWithParameterIndexWithPG(ctx, v.CallExpr.Args[0], paramIndex)
			if err != nil {
				return paramIndex, err
			}
			if _, err := ctx.Buffer.WriteString(")"); err != nil {
				return paramIndex, err
			}
			return newParamIndex, nil
		case "_==_", "_!=_", "_<_", "_>_", "_<=_", "_>=_":
			if len(v.CallExpr.Args) != 2 {
				return paramIndex, fmt.Errorf("invalid number of arguments for %s", v.CallExpr.Function)
			}
			// Check if the left side is a function call like size(tags)
			if leftCallExpr, ok := v.CallExpr.Args[0].ExprKind.(*exprv1.Expr_CallExpr); ok {
				if leftCallExpr.CallExpr.Function == "size" {
					// Handle size(tags) comparison
					if len(leftCallExpr.CallExpr.Args) != 1 {
						return paramIndex, errors.New("size function requires exactly one argument")
					}
					identifier, err := cel.GetIdentExprName(leftCallExpr.CallExpr.Args[0])
					if err != nil {
						return paramIndex, err
					}
					if !slices.Contains(cel.ColumnsSizeable, identifier) {
						return paramIndex, errors.Errorf("size function not supports '%s' identifier", identifier)
					}
					value, err := cel.GetExprValue(v.CallExpr.Args[1])
					if err != nil {
						return paramIndex, err
					}
					valueInt, ok := value.(int64)
					if !ok {
						return paramIndex, errors.New("size comparison value must be an integer")
					}
					operator := getComparisonOperatorWithPG(v.CallExpr.Function)

					if _, err := ctx.Buffer.WriteString(fmt.Sprintf("%s %s %s",
						cel.GetSQL("json_array_length", cel.PostgreSQLTemplate, identifier), operator,
						cel.GetParameterPlaceholder(cel.PostgreSQLTemplate, paramIndex))); err != nil {
						return paramIndex, err
					}
					ctx.Args = append(ctx.Args, valueInt)
					return paramIndex + 1, nil
				}
			}

			identifier, err := cel.GetIdentExprName(v.CallExpr.Args[0])
			if err != nil {
				return paramIndex, err
			}
			value, err := cel.GetExprValue(v.CallExpr.Args[1])
			if err != nil {
				return paramIndex, err
			}
			if err = cel.CheckValueType(identifier, value); err != nil {
				return paramIndex, fmt.Errorf("invalid type for %s", v.CallExpr.Function)
			}

			operator := getComparisonOperatorWithPG(v.CallExpr.Function)

			if slices.Contains(cel.ColumnsTime, identifier) {
				valueInt, ok := value.(int64)
				if !ok {
					return paramIndex, errors.New("invalid integer timestamp value")
				}

				timestampSQL := cel.GetSQL("timestamp_field", cel.PostgreSQLTemplate, identifier)
				if _, err := ctx.Buffer.WriteString(fmt.Sprintf("%s %s %s", timestampSQL, operator,
					cel.GetParameterPlaceholder(cel.PostgreSQLTemplate, paramIndex))); err != nil {
					return paramIndex, err
				}
				ctx.Args = append(ctx.Args, valueInt)
				return paramIndex + 1, nil
			} else if slices.Contains(cel.ColumnsBool, identifier) {
				if operator != "=" && operator != "!=" {
					return paramIndex, fmt.Errorf("invalid operator for %s", v.CallExpr.Function)
				}
				valueBool, ok := value.(bool)
				if !ok {
					return paramIndex, errors.New("invalid boolean value for has_task_list")
				}
				// Use parameterized template for boolean comparison (PostgreSQL only)
				placeholder := cel.GetParameterPlaceholder(cel.PostgreSQLTemplate, paramIndex)
				sqlTemplate := cel.GetSQL("boolean_compare", cel.PostgreSQLTemplate, operator)
				sqlTemplate = strings.Replace(sqlTemplate, "?", placeholder, 1)
				if _, err := ctx.Buffer.WriteString(sqlTemplate); err != nil {
					return paramIndex, err
				}
				ctx.Args = append(ctx.Args, valueBool)
				return paramIndex + 1, nil
			} else {
				if operator != "=" && operator != "!=" {
					return paramIndex, fmt.Errorf("invalid operator for %s", v.CallExpr.Function)
				}

				sqlTemplate := cel.GetSQL("table_prefix", cel.PostgreSQLTemplate, identifier)
				if _, err := ctx.Buffer.WriteString(fmt.Sprintf("%s %s %s", sqlTemplate, operator,
					cel.GetParameterPlaceholder(cel.PostgreSQLTemplate, paramIndex))); err != nil {
					return paramIndex, err
				}
				ctx.Args = append(ctx.Args, value)
				return paramIndex + 1, nil
			}
		case "@in":
			if len(v.CallExpr.Args) != 2 {
				return paramIndex, fmt.Errorf("invalid number of arguments for %s", v.CallExpr.Function)
			}

			// Check if this is "element in collection" syntax
			if identifier, err := cel.GetIdentExprName(v.CallExpr.Args[1]); err == nil {
				// This is "element in collection" - the second argument is the collection
				if !slices.Contains(cel.ColumnsList, identifier) {
					return paramIndex, fmt.Errorf("invalid collection identifier for %s: %s", v.CallExpr.Function, identifier)
				}

				// Handle "element" in tags
				element, err := cel.GetConstValue(v.CallExpr.Args[0])
				if err != nil {
					return paramIndex, fmt.Errorf("first argument must be a constant value for 'element': %v", err)
				}
				placeholder := cel.GetParameterPlaceholder(cel.PostgreSQLTemplate, paramIndex)
				sql := strings.Replace(cel.GetSQL("json_contains_element", cel.PostgreSQLTemplate, identifier), "?", placeholder, 1)
				if _, err := ctx.Buffer.WriteString(sql); err != nil {
					return paramIndex, err
				}
				ctx.Args = append(ctx.Args, cel.GetParameterValue(cel.PostgreSQLTemplate, "json_contains_element", element))
				return paramIndex + 1, nil
			}

			// Original logic for "identifier in [list]" syntax
			identifier, err := cel.GetIdentExprName(v.CallExpr.Args[0])
			if err != nil {
				return paramIndex, err
			}
			if !slices.Contains(cel.ColumnsList, identifier) {
				return paramIndex, fmt.Errorf("invalid identifier for %s", v.CallExpr.Function)
			}

			values := []any{}
			for _, element := range v.CallExpr.Args[1].GetListExpr().Elements {
				value, err := cel.GetConstValue(element)
				if err != nil {
					return paramIndex, err
				}
				values = append(values, value)
			}
			if slices.Contains(cel.ColumnsList, identifier) {
				subconditions := []string{}
				args := []any{}
				currentParamIndex := paramIndex
				for _, v := range values {
					// Use parameter index for each placeholder
					placeholder := cel.GetParameterPlaceholder(cel.PostgreSQLTemplate, currentParamIndex)
					subcondition := strings.Replace(cel.GetSQL("json_contains_element", cel.PostgreSQLTemplate, identifier), "?", placeholder, 1)
					subconditions = append(subconditions, subcondition)
					args = append(args, cel.GetParameterValue(cel.PostgreSQLTemplate, "json_contains_element", v))
					currentParamIndex++
				}
				if len(subconditions) == 1 {
					if _, err := ctx.Buffer.WriteString(subconditions[0]); err != nil {
						return paramIndex, err
					}
				} else {
					if _, err := ctx.Buffer.WriteString(fmt.Sprintf("(%s)", strings.Join(subconditions, " OR "))); err != nil {
						return paramIndex, err
					}
				}
				ctx.Args = append(ctx.Args, args...)
				return paramIndex + len(args), nil
			} else {
				placeholders := cel.FormatPlaceholders(cel.PostgreSQLTemplate, len(values), paramIndex)
				visibilitySQL := fmt.Sprintf(cel.GetSQL("content_in", cel.PostgreSQLTemplate, identifier), strings.Join(placeholders, ","))
				if _, err := ctx.Buffer.WriteString(visibilitySQL); err != nil {
					return paramIndex, err
				}
				ctx.Args = append(ctx.Args, values...)
				return paramIndex + len(values), nil
			}
		case "contains":
			if len(v.CallExpr.Args) != 1 {
				return paramIndex, fmt.Errorf("invalid number of arguments for %s", v.CallExpr.Function)
			}
			identifier, err := cel.GetIdentExprName(v.CallExpr.Target)
			if err != nil {
				return paramIndex, err
			}
			if identifier != "content" {
				return paramIndex, fmt.Errorf("invalid identifier for %s", v.CallExpr.Function)
			}
			arg, err := cel.GetConstValue(v.CallExpr.Args[0])
			if err != nil {
				return paramIndex, err
			}
			placeholder := cel.GetParameterPlaceholder(cel.PostgreSQLTemplate, paramIndex)
			sql := strings.Replace(cel.GetSQL("content_like", cel.PostgreSQLTemplate, identifier), "?", placeholder, 1)
			if _, err := ctx.Buffer.WriteString(sql); err != nil {
				return paramIndex, err
			}
			ctx.Args = append(ctx.Args, fmt.Sprintf("%%%s%%", arg))
			return paramIndex + 1, nil
		}
	} else if v, ok := expr.ExprKind.(*exprv1.Expr_IdentExpr); ok {
		identifier := v.IdentExpr.GetName()
		if !slices.Contains(cel.ColumnsBool, identifier) {
			return paramIndex, fmt.Errorf("invalid identifier %s", identifier)
		}
		if _, err := ctx.Buffer.WriteString(cel.GetSQL("boolean_check", cel.PostgreSQLTemplate, identifier)); err != nil {
			return paramIndex, err
		}
	}
	return paramIndex, nil
}

func getComparisonOperatorWithPG(function string) string {
	switch function {
	case "_==_":
		return "="
	case "_!=_":
		return "!="
	case "_<_":
		return "<"
	case "_>_":
		return ">"
	case "_<=_":
		return "<="
	case "_>=_":
		return ">="
	default:
		return "="
	}
}
