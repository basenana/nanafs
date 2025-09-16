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
	"github.com/basenana/nanafs/pkg/cel"
	"testing"
)

func Test_convertWithParameterIndexWithPG(t *testing.T) {
	type args struct {
		filter string
		wanted string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "DB Column Match",
			args: args{
				filter: `name == "test"`,
				wanted: "children.name = ?",
			},
		},
		//{
		//	name: "DB Time Match",
		//	args: args{
		//		filter: `changed_at <= now()"`,
		//		wanted: "",
		//	},
		//},
		{
			name: "DB JSON Column Key Match",
			args: args{
				filter: `group.source == "rss"`,
				wanted: "egroup.value->>'source' = ?",
			},
		},
		{
			name: "DB JSON Column List Match",
			args: args{
				filter: `"test" in tags`,
				wanted: "property.value->'tags' @> jsonb_build_array(?)",
			},
		},
		{
			name: "DB JSON Column List Match",
			args: args{
				filter: `tag in ["test1", "test2"]`,
				wanted: "(property.value->'tags' @> jsonb_build_array(?) OR property.value->'tags' @> jsonb_build_array(?))",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedExpr, err := cel.Parse(tt.args.filter)
			if err != nil {
				t.Errorf("pares ce filter error = %v", err)
				return
			}

			ctx := NewConvertContext()
			if err := convertWithParameterIndexWithPG(ctx, parsedExpr.Expr); err != nil {
				t.Errorf("convertWithTemplatesWithSQLite() error = %v", err)
				return
			}

			got := ctx.Buffer.String()
			if got != tt.args.wanted {
				t.Errorf("convertWithTemplatesWithSQLite() = %v, want %v", got, tt.args.wanted)
				return
			}
		})
	}
}
