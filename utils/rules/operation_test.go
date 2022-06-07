package rules

import (
	"github.com/basenana/nanafs/pkg/types"
	"testing"
	"time"
)

func TestRule_Apply(t *testing.T) {
	type fields struct {
		Logic     string
		Rules     []types.RuleCondition
		Operation RuleOperation
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
			fields: fields{Operation: Equal{ColumnKey: "name", Content: "abc"}},
			args:   args{value: &types.Object{Metadata: types.Metadata{ID: "abc", Name: "abc"}}},
			want:   true,
		},
		{
			name:   "test-not-equal",
			fields: fields{Operation: Equal{ColumnKey: "name", Content: "abc"}},
			args:   args{value: &types.Object{Metadata: types.Metadata{Name: "aaa"}}},
			want:   false,
		},
		{
			name:   "test-pattern",
			fields: fields{Operation: Pattern{ColumnKey: "name", Content: "[a-z]+"}},
			args:   args{value: &types.Object{Metadata: types.Metadata{Name: "aaa"}}},
			want:   true,
		},
		{
			name:   "test-not-pattern",
			fields: fields{Operation: Pattern{ColumnKey: "name", Content: "[a-z]+"}},
			args:   args{value: &types.Object{Metadata: types.Metadata{Name: "AAA"}}},
			want:   false,
		},
		{
			name:   "test-in",
			fields: fields{Operation: In{ColumnKey: "name", Content: []string{"aaa", "bbb"}}},
			args:   args{value: &types.Object{Metadata: types.Metadata{Name: "aaa"}}},
			want:   true,
		},
		{
			name:   "test-not-in",
			fields: fields{Operation: In{ColumnKey: "name", Content: []string{"aaa", "bbb"}}},
			args:   args{value: &types.Object{Metadata: types.Metadata{Name: "abc"}}},
			want:   false,
		},
		{
			name:   "test-before",
			fields: fields{Operation: Before{ColumnKey: "created_at", Content: time.Now().AddDate(1, 1, 1)}},
			args:   args{value: &types.Object{Metadata: types.Metadata{CreatedAt: time.Now()}}},
			want:   true,
		},
		{
			name:   "test-not-before",
			fields: fields{Operation: Before{ColumnKey: "created_at", Content: time.Now()}},
			args:   args{value: &types.Object{Metadata: types.Metadata{CreatedAt: time.Now().AddDate(1, 1, 1)}}},
			want:   false,
		},
		{
			name:   "test-after",
			fields: fields{Operation: After{ColumnKey: "created_at", Content: time.Now()}},
			args:   args{value: &types.Object{Metadata: types.Metadata{CreatedAt: time.Now().AddDate(1, 1, 1)}}},
			want:   true,
		},
		{
			name:   "test-not-after",
			fields: fields{Operation: After{ColumnKey: "created_at", Content: time.Now().AddDate(1, 1, 1)}},
			args:   args{value: &types.Object{Metadata: types.Metadata{CreatedAt: time.Now()}}},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := types.RuleCondition{
				Logic:     tt.fields.Logic,
				Rules:     tt.fields.Rules,
				Operation: tt.fields.Operation,
			}
			if got := r.Apply(tt.args.value); got != tt.want {
				t.Errorf("Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}
