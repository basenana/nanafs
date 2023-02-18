package group

import (
	"github.com/basenana/nanafs/pkg/types"
	"testing"
	"time"
)

func TestRule_Apply(t *testing.T) {
	type fields struct {
		Logic     types.RuleFilterLogic
		Rules     []types.RuleFilter
		Operation types.RuleOperation
		Column    string
		Value     string
	}
	type args struct {
		value *types.Object
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "test-nil",
			fields: fields{},
			args:   args{},
			want:   false,
		},
		{
			name:   "test-equal",
			fields: fields{Operation: "equal", Column: "name", Value: "abc"},
			args:   args{value: &types.Object{Metadata: types.Metadata{ID: 1024, Name: "abc"}}},
			want:   true,
		},
		{
			name:   "test-not-equal",
			fields: fields{Operation: "equal", Column: "name", Value: "abc"},
			args:   args{value: &types.Object{Metadata: types.Metadata{Name: "aaa"}}},
			want:   false,
		},
		{
			name:   "test-pattern",
			fields: fields{Operation: "pattern", Column: "name", Value: "[a-z]+"},
			args:   args{value: &types.Object{Metadata: types.Metadata{Name: "aaa"}}},
			want:   true,
		},
		{
			name:   "test-not-pattern",
			fields: fields{Operation: "pattern", Column: "name", Value: "[a-z]+"},
			args:   args{value: &types.Object{Metadata: types.Metadata{Name: "AAA"}}},
			want:   false,
		},
		{
			name:   "test-in",
			fields: fields{Operation: "in", Column: "name", Value: "aaa,bbb"},
			args:   args{value: &types.Object{Metadata: types.Metadata{Name: "aaa"}}},
			want:   true,
		},
		{
			name:   "test-not-in",
			fields: fields{Operation: "in", Column: "name", Value: "aaa,bbb"},
			args:   args{value: &types.Object{Metadata: types.Metadata{Name: "abc"}}},
			want:   false,
		},
		{
			name:   "test-before",
			fields: fields{Operation: "before", Column: "created_at", Value: time.Now().AddDate(1, 1, 1).Format(timeOpFmt)},
			args:   args{value: &types.Object{Metadata: types.Metadata{CreatedAt: time.Now()}}},
			want:   true,
		},
		{
			name:   "test-not-before",
			fields: fields{Operation: "before", Column: "created_at", Value: time.Now().Format(timeOpFmt)},
			args:   args{value: &types.Object{Metadata: types.Metadata{CreatedAt: time.Now().AddDate(1, 1, 1)}}},
			want:   false,
		},
		{
			name:   "test-after",
			fields: fields{Operation: "after", Column: "created_at", Value: time.Now().Format(timeOpFmt)},
			args:   args{value: &types.Object{Metadata: types.Metadata{CreatedAt: time.Now().AddDate(1, 1, 1)}}},
			want:   true,
		},
		{
			name:   "test-not-after",
			fields: fields{Operation: "after", Column: "created_at", Value: time.Now().AddDate(1, 1, 1).Format(timeOpFmt)},
			args:   args{value: &types.Object{Metadata: types.Metadata{CreatedAt: time.Now()}}},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := types.RuleFilter{
				Logic:     tt.fields.Logic,
				Filters:   tt.fields.Rules,
				Operation: tt.fields.Operation,
				Column:    tt.fields.Column,
				Value:     tt.fields.Value,
			}
			if got := objectFilter(r, tt.args.value); got != tt.want {
				t.Errorf("Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}
