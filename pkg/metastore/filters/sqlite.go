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

package filters

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/cel"
	"github.com/pkg/errors"
	exprv1 "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"slices"
	"strings"
)

func convertWithTemplatesWithSQLite(ctx *ConvertContext, expr *exprv1.Expr) error {
	const dbType = cel.SQLiteTemplate

	if v, ok := expr.ExprKind.(*exprv1.Expr_CallExpr); ok {
		switch v.CallExpr.Function {
		case "_||_", "_&&_":
			if len(v.CallExpr.Args) != 2 {
				return errors.Errorf("invalid number of arguments for %s", v.CallExpr.Function)
			}
			if _, err := ctx.Buffer.WriteString("("); err != nil {
				return err
			}
			if err := convertWithTemplatesWithSQLite(ctx, v.CallExpr.Args[0]); err != nil {
				return err
			}
			operator := "AND"
			if v.CallExpr.Function == "_||_" {
				operator = "OR"
			}
			if _, err := ctx.Buffer.WriteString(fmt.Sprintf(" %s ", operator)); err != nil {
				return err
			}
			if err := convertWithTemplatesWithSQLite(ctx, v.CallExpr.Args[1]); err != nil {
				return err
			}
			if _, err := ctx.Buffer.WriteString(")"); err != nil {
				return err
			}
		case "!_":
			if len(v.CallExpr.Args) != 1 {
				return errors.Errorf("invalid number of arguments for %s", v.CallExpr.Function)
			}
			if _, err := ctx.Buffer.WriteString("NOT ("); err != nil {
				return err
			}
			if err := convertWithTemplatesWithSQLite(ctx, v.CallExpr.Args[0]); err != nil {
				return err
			}
			if _, err := ctx.Buffer.WriteString(")"); err != nil {
				return err
			}
		case "_==_", "_!=_", "_<_", "_>_", "_<=_", "_>=_":
			if len(v.CallExpr.Args) != 2 {
				return errors.Errorf("invalid number of arguments for %s", v.CallExpr.Function)
			}
			// Check if the left side is a function call like size(tags)
			if leftCallExpr, ok := v.CallExpr.Args[0].ExprKind.(*exprv1.Expr_CallExpr); ok {
				if leftCallExpr.CallExpr.Function == "size" {
					// Handle size(tags) comparison
					if len(leftCallExpr.CallExpr.Args) != 1 {
						return errors.New("size function requires exactly one argument")
					}
					identifier, err := cel.GetIdentExprName(leftCallExpr.CallExpr.Args[0])
					if err != nil {
						return err
					}
					if !slices.Contains(cel.ColumnsSizeable, identifier) {
						return errors.Errorf("size function only supports 'tags' identifier, got: %s", identifier)
					}
					value, err := cel.GetExprValue(v.CallExpr.Args[1])
					if err != nil {
						return err
					}
					valueInt, ok := value.(int64)
					if !ok {
						return errors.New("size comparison value must be an integer")
					}
					operator := getComparisonOperatorWithSQLite(v.CallExpr.Function)

					if _, err := ctx.Buffer.WriteString(fmt.Sprintf("%s %s ?",
						cel.GetSQL("array_length", dbType, identifier), operator)); err != nil {
						return err
					}
					ctx.Args = append(ctx.Args, valueInt)
					return nil
				}
			}

			identifier, err := cel.GetIdentExprName(v.CallExpr.Args[0])
			if err != nil {
				return err
			}
			if !slices.Contains(cel.ColumnsComparable, identifier) {
				return errors.Errorf("invalid identifier for %s", identifier)
			}
			value, err := cel.GetExprValue(v.CallExpr.Args[1])
			if err != nil {
				return err
			}
			operator := getComparisonOperatorWithSQLite(v.CallExpr.Function)

			if slices.Contains(cel.ColumnsTime, identifier) {
				valueInt, ok := value.(int64)
				if !ok {
					return errors.New("invalid integer timestamp value")
				}

				timestampSQL := cel.GetSQL("timestamp_field", dbType, identifier)
				if _, err := ctx.Buffer.WriteString(fmt.Sprintf("%s %s ?", timestampSQL, operator)); err != nil {
					return err
				}
				ctx.Args = append(ctx.Args, valueInt)
			} else if slices.Contains(cel.ColumnsBool, identifier) {
				if operator != "=" && operator != "!=" {
					return errors.Errorf("invalid operator for %s", v.CallExpr.Function)
				}
				valueBool, ok := value.(bool)
				if !ok {
					return errors.New("invalid boolean value for has_task_list")
				}
				// Use template for boolean comparison
				sqlTemplate := cel.GetSQL("boolean_compare", dbType, identifier, operator)
				if _, err := ctx.Buffer.WriteString(sqlTemplate); err != nil {
					return err
				}
				ctx.Args = append(ctx.Args, valueBool)
			} else {
				if operator != "=" && operator != "!=" {
					return errors.Errorf("invalid operator for %s", v.CallExpr.Function)
				}

				if err = cel.CheckValueType(identifier, value); err != nil {
					return errors.Errorf("invalid value for %s %s", identifier, err)
				}

				sqlTemplate := cel.GetSQL("content_compare", dbType, identifier, operator)
				if _, err := ctx.Buffer.WriteString(sqlTemplate); err != nil {
					return err
				}
				ctx.Args = append(ctx.Args, value)
			}
		case "@in":
			if len(v.CallExpr.Args) != 2 {
				return errors.Errorf("invalid number of arguments for %s", v.CallExpr.Function)
			}

			// Check if this is "element in collection" syntax
			if identifier, err := cel.GetIdentExprName(v.CallExpr.Args[1]); err == nil {
				// This is "element in collection" - the second argument is the collection
				if !slices.Contains(cel.ColumnsList, identifier) {
					return errors.Errorf("invalid collection identifier for %s: %s", v.CallExpr.Function, identifier)
				}

				if slices.Contains(cel.ColumnsList, identifier) {
					// Handle "element" in tags
					element, err := cel.GetConstValue(v.CallExpr.Args[0])
					if err != nil {
						return errors.Errorf("first argument must be a constant value for 'element in %s': %v", err, identifier)
					}
					if _, err := ctx.Buffer.WriteString(cel.GetSQL("contains_element", dbType, identifier)); err != nil {
						return err
					}
					ctx.Args = append(ctx.Args, cel.GetParameterValue(dbType, "contains_element", element))
				}
				return nil
			}

			// Original logic for "identifier in [list]" syntax
			identifier, err := cel.GetIdentExprName(v.CallExpr.Args[0])
			if err != nil {
				return err
			}
			if !slices.Contains(cel.ColumnsList, identifier) {
				return errors.Errorf("invalid identifier for %s", v.CallExpr.Function)
			}

			values := []any{}
			for _, element := range v.CallExpr.Args[1].GetListExpr().Elements {
				value, err := cel.GetConstValue(element)
				if err != nil {
					return err
				}
				values = append(values, value)
			}
			if slices.Contains(cel.ColumnsList, identifier) {
				subconditions := []string{}
				args := []any{}
				for _, v := range values {
					subconditions = append(subconditions, cel.GetSQL("contains_element", dbType, identifier))
					args = append(args, cel.GetParameterValue(dbType, "contains_element", v))
				}
				if len(subconditions) == 1 {
					if _, err := ctx.Buffer.WriteString(subconditions[0]); err != nil {
						return err
					}
				} else {
					if _, err := ctx.Buffer.WriteString(fmt.Sprintf("(%s)", strings.Join(subconditions, " OR "))); err != nil {
						return err
					}
				}
				ctx.Args = append(ctx.Args, args...)
			} else {
				placeholders := cel.FormatPlaceholders(dbType, len(values), 1)
				visibilitySQL := cel.GetSQL("content_in", dbType, identifier, strings.Join(placeholders, ","))
				if _, err := ctx.Buffer.WriteString(visibilitySQL); err != nil {
					return err
				}
				ctx.Args = append(ctx.Args, values...)
			}
		case "contains":
			if len(v.CallExpr.Args) != 1 {
				return errors.Errorf("invalid number of arguments for %s", v.CallExpr.Function)
			}
			identifier, err := cel.GetIdentExprName(v.CallExpr.Target)
			if err != nil {
				return err
			}
			arg, err := cel.GetConstValue(v.CallExpr.Args[0])
			if err != nil {
				return err
			}
			if err = cel.CheckValueType(identifier, arg); err != nil {
				return errors.Errorf("invalid value for %s %s", v.CallExpr.Function, identifier)
			}

			if _, err := ctx.Buffer.WriteString(cel.GetSQL("content_like", dbType, identifier)); err != nil {
				return err
			}
			ctx.Args = append(ctx.Args, fmt.Sprintf("%%%s%%", arg))
		}
	} else if v, ok := expr.ExprKind.(*exprv1.Expr_IdentExpr); ok {
		identifier := v.IdentExpr.GetName()
		if !slices.Contains(cel.ColumnsBool, identifier) {
			return errors.Errorf("invalid identifier %s", identifier)
		}
		// Handle has_task_list as a standalone boolean identifier
		if _, err := ctx.Buffer.WriteString(cel.GetSQL("boolean_check", dbType, identifier)); err != nil {
			return err
		}
	}
	return nil
}

func getComparisonOperatorWithSQLite(function string) string {
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
