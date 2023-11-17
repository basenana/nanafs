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
	"time"
)

func TestRule_Apply(t *testing.T) {
	type fields struct {
		Logic     string
		Rules     []types.Rule
		Operation string
		Column    string
		Value     string
	}
	type args struct {
		value *object
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "test-equal",
			fields: fields{Operation: types.RuleOpEqual, Column: "name", Value: "abc"},
			args:   args{value: &object{Metadata: &types.Metadata{ID: 1024, Name: "abc"}}},
			want:   true,
		},
		{
			name:   "test-not-equal",
			fields: fields{Operation: types.RuleOpEqual, Column: "name", Value: "abc"},
			args:   args{value: &object{Metadata: &types.Metadata{Name: "aaa"}}},
			want:   false,
		},
		{
			name:   "test-pattern",
			fields: fields{Operation: types.RuleOpPattern, Column: "name", Value: "[a-z]+"},
			args:   args{value: &object{Metadata: &types.Metadata{Name: "aaa"}}},
			want:   true,
		},
		{
			name:   "test-not-pattern",
			fields: fields{Operation: types.RuleOpPattern, Column: "name", Value: "[a-z]+"},
			args:   args{value: &object{Metadata: &types.Metadata{Name: "AAA"}}},
			want:   false,
		},
		{
			name:   "test-in",
			fields: fields{Operation: types.RuleOpIn, Column: "name", Value: "aaa,bbb"},
			args:   args{value: &object{Metadata: &types.Metadata{Name: "aaa"}}},
			want:   true,
		},
		{
			name:   "test-not-in",
			fields: fields{Operation: types.RuleOpIn, Column: "name", Value: "aaa,bbb"},
			args:   args{value: &object{Metadata: &types.Metadata{Name: "abc"}}},
			want:   false,
		},
		{
			name:   "test-before",
			fields: fields{Operation: types.RuleOpBefore, Column: "created_at", Value: time.Now().AddDate(1, 1, 1).Format(timeOpFmt)},
			args:   args{value: &object{Metadata: &types.Metadata{CreatedAt: time.Now()}}},
			want:   true,
		},
		{
			name:   "test-not-before",
			fields: fields{Operation: types.RuleOpBefore, Column: "created_at", Value: time.Now().Format(timeOpFmt)},
			args:   args{value: &object{Metadata: &types.Metadata{CreatedAt: time.Now().AddDate(1, 1, 1)}}},
			want:   false,
		},
		{
			name:   "test-after",
			fields: fields{Operation: types.RuleOpAfter, Column: "created_at", Value: time.Now().Format(timeOpFmt)},
			args:   args{value: &object{Metadata: &types.Metadata{CreatedAt: time.Now().AddDate(1, 1, 1)}}},
			want:   true,
		},
		{
			name:   "test-not-after",
			fields: fields{Operation: types.RuleOpAfter, Column: "created_at", Value: time.Now().AddDate(1, 1, 1).Format(timeOpFmt)},
			args:   args{value: &object{Metadata: &types.Metadata{CreatedAt: time.Now()}}},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := types.Rule{
				Logic:     tt.fields.Logic,
				Rules:     tt.fields.Rules,
				Operation: tt.fields.Operation,
				Column:    tt.fields.Column,
				Value:     tt.fields.Value,
			}
			if got := NewRuleOperation(r.Operation, r.Column, r.Value).
				Apply(entryToMap(tt.args.value.Metadata, tt.args.value.ExtendData)); got != tt.want {
				t.Errorf("Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_labelOperation(t *testing.T) {
	type args struct {
		labels *types.Labels
		match  *types.LabelMatch
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-all-include",
			args: args{
				labels: &types.Labels{Labels: []types.Label{
					{Key: "test-key-1", Value: "test-val-1"},
					{Key: "test-key-2", Value: "test-val-2"},
					{Key: "test-key-3", Value: "test-val-3"},
				}},
				match: &types.LabelMatch{
					Include: []types.Label{
						{Key: "test-key-1", Value: "test-val-1"},
						{Key: "test-key-2", Value: "test-val-2"},
					},
					Exclude: []string{},
				},
			},
			want: true,
		},
		{
			name: "test-some-include",
			args: args{
				labels: &types.Labels{Labels: []types.Label{
					{Key: "test-key-1", Value: "test-val-1"},
					{Key: "test-key-2", Value: "test-val-2"},
					{Key: "test-key-3", Value: "test-val-3"},
				}},
				match: &types.LabelMatch{
					Include: []types.Label{
						{Key: "test-key-3", Value: "test-val-3"},
						{Key: "test-key-4", Value: "test-val-4"},
					},
					Exclude: []string{},
				},
			},
			want: false,
		},
		{
			name: "test-no-include",
			args: args{
				labels: &types.Labels{Labels: []types.Label{
					{Key: "test-key-1", Value: "test-val-1"},
					{Key: "test-key-2", Value: "test-val-2"},
					{Key: "test-key-3", Value: "test-val-3"},
				}},
				match: &types.LabelMatch{
					Include: []types.Label{
						{Key: "test-key-4", Value: "test-val-4"},
					},
					Exclude: []string{},
				},
			},
			want: false,
		},
		{
			name: "test-has-exclude1",
			args: args{
				labels: &types.Labels{Labels: []types.Label{
					{Key: "test-key-1", Value: "test-val-1"},
					{Key: "test-key-2", Value: "test-val-2"},
					{Key: "test-key-3", Value: "test-val-3"},
				}},
				match: &types.LabelMatch{
					Include: []types.Label{},
					Exclude: []string{"test-key-4"},
				},
			},
			want: true,
		},
		{
			name: "test-has-exclude1",
			args: args{
				labels: &types.Labels{Labels: []types.Label{
					{Key: "test-key-1", Value: "test-val-1"},
					{Key: "test-key-2", Value: "test-val-2"},
					{Key: "test-key-3", Value: "test-val-3"},
				}},
				match: &types.LabelMatch{
					Include: []types.Label{
						{Key: "test-key-1", Value: "test-val-1"},
						{Key: "test-key-3", Value: "test-val-3"},
					},
					Exclude: []string{"test-key-4"},
				},
			},
			want: true,
		},
		{
			name: "test-has-exclude3",
			args: args{
				labels: &types.Labels{Labels: []types.Label{
					{Key: "test-key-1", Value: "test-val-1"},
					{Key: "test-key-2", Value: "test-val-2"},
					{Key: "test-key-3", Value: "test-val-3"},
				}},
				match: &types.LabelMatch{
					Include: []types.Label{},
					Exclude: []string{"test-key-3"},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := labelOperation(tt.args.labels, tt.args.match); got != tt.want {
				t.Errorf("labelOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}
