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
	"testing"
)

type object struct {
	*types.Entry
	*types.Properties
	*types.Labels
}

func TestObjectFilter(t *testing.T) {
	obj := &object{
		Entry: &types.Entry{ID: 1024, Name: "test_file_1.txt"},
		Labels: &types.Labels{Labels: []types.Label{
			{Key: "test-key-1", Value: "test-val-1"},
			{Key: "test-key-2", Value: "test-val-2"},
			{Key: "test-key-3", Value: "test-val-3"},
		}},
	}

	tests := []struct {
		name   string
		filter *types.Rule
		want   bool
	}{
		{
			name: "test-all-need-true",
			filter: &types.Rule{
				Logic: types.RuleLogicAll,
				Rules: []types.Rule{
					{
						Operation: types.RuleOpEqual,
						Column:    "name",
						Value:     "test_file_1.txt",
						Labels:    nil,
					},
					{
						Operation: types.RuleOpEndWith,
						Column:    "name",
						Value:     "txt",
					},
					{
						Labels: &types.LabelMatch{
							Include: []types.Label{
								{Key: "test-key-1", Value: "test-val-1"},
								{Key: "test-key-2", Value: "test-val-2"},
							},
							Exclude: []string{},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test-all-need-false",
			filter: &types.Rule{
				Logic: types.RuleLogicAll,
				Rules: []types.Rule{
					{
						Operation: types.RuleOpEqual,
						Column:    "name",
						Value:     "test_file_none",
						Labels:    nil,
					},
					{
						Labels: &types.LabelMatch{
							Include: []types.Label{
								{Key: "test-key-1", Value: "test-val-1"},
								{Key: "test-key-2", Value: "test-val-2"},
							},
							Exclude: []string{},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "test-or-need-true",
			filter: &types.Rule{
				Logic: types.RuleLogicAny,
				Rules: []types.Rule{
					{
						Operation: types.RuleOpEqual,
						Column:    "name",
						Value:     "test_file_none",
						Labels:    nil,
					},
					{
						Labels: &types.LabelMatch{
							Include: []types.Label{
								{Key: "test-key-1", Value: "test-val-1"},
								{Key: "test-key-2", Value: "test-val-2"},
							},
							Exclude: []string{},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test-or-need-false",
			filter: &types.Rule{
				Logic: types.RuleLogicAny,
				Rules: []types.Rule{
					{
						Operation: types.RuleOpEqual,
						Column:    "name",
						Value:     "test_file_none",
						Labels:    nil,
					},
					{
						Labels: &types.LabelMatch{
							Include: []types.Label{
								{Key: "test-key-1", Value: "test-val-3"},
								{Key: "test-key-2", Value: "test-val-2"},
							},
							Exclude: []string{},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "test-not-need-true",
			filter: &types.Rule{
				Logic: types.RuleLogicNot,
				Rules: []types.Rule{
					{
						Operation: types.RuleOpEqual,
						Column:    "name",
						Value:     "test_file_none",
						Labels:    nil,
					},
					{
						Labels: &types.LabelMatch{
							Include: []types.Label{
								{Key: "test-key-1", Value: "test-val-3"},
								{Key: "test-key-2", Value: "test-val-2"},
							},
							Exclude: []string{},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test-not-need-false",
			filter: &types.Rule{
				Logic: types.RuleLogicNot,
				Rules: []types.Rule{
					{
						Operation: types.RuleOpEqual,
						Column:    "name",
						Value:     "test_file_none",
						Labels:    nil,
					},
					{
						Labels: &types.LabelMatch{
							Include: []types.Label{
								{Key: "test-key-1", Value: "test-val-1"},
								{Key: "test-key-2", Value: "test-val-2"},
							},
							Exclude: []string{},
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Filter(tt.filter, obj.Entry, obj.Properties, obj.Labels); got != tt.want {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}
