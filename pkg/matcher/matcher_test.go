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

package matcher

import (
	"context"
	"testing"
	"time"

	"github.com/basenana/nanafs/pkg/types"
)

func TestEntryMatch(t *testing.T) {
	var (
		ctx   = context.Background()
		entry = &types.Entry{
			ID:         1,
			Name:       "test.txt",
			Kind:       types.TextKind,
			Size:       1000,
			IsGroup:    false,
			Namespace:  "default",
			CreatedAt:  time.Now(),
			ChangedAt:  time.Now(),
			ModifiedAt: time.Now(),
			AccessAt:   time.Now(),
		}
		match *types.WorkflowEntryMatch
	)

	match = &types.WorkflowEntryMatch{}
	matched, err := EntryMatch(ctx, entry, match)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !matched {
		t.Error("expected to match when no conditions set")
	}

	// FileNamePattern tests
	t.Run("FileNamePattern exact match", func(t *testing.T) {
		match.FileNamePattern = "test.txt"
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !matched {
			t.Error("expected to match exact name")
		}
	})

	t.Run("FileNamePattern glob *", func(t *testing.T) {
		match.FileNamePattern = "*.txt"
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !matched {
			t.Error("expected to match glob pattern")
		}
	})

	t.Run("FileNamePattern no match", func(t *testing.T) {
		match.FileNamePattern = "*.pdf"
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if matched {
			t.Error("expected not to match")
		}
	})

	// FileTypes tests
	t.Run("FileTypes single", func(t *testing.T) {
		match.FileNamePattern = ""
		match.FileTypes = "txt"
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !matched {
			t.Error("expected to match file type")
		}
	})

	t.Run("FileTypes multiple", func(t *testing.T) {
		match.FileTypes = "txt,pdf,md"
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !matched {
			t.Error("expected to match multiple file types")
		}
	})

	t.Run("FileTypes no match", func(t *testing.T) {
		match.FileTypes = "pdf"
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if matched {
			t.Error("expected not to match")
		}
	})

	// FileSize tests
	t.Run("FileSize in range", func(t *testing.T) {
		match.FileTypes = ""
		match.MinFileSize = 500
		match.MaxFileSize = 2000
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !matched {
			t.Error("expected to match size in range")
		}
	})

	t.Run("FileSize below min", func(t *testing.T) {
		match.MinFileSize = 2000
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if matched {
			t.Error("expected not to match when below min")
		}
	})

	t.Run("FileSize above max", func(t *testing.T) {
		match.MinFileSize = 0
		match.MaxFileSize = 500
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if matched {
			t.Error("expected not to match when above max")
		}
	})

	// CEL pattern tests
	t.Run("CEL pattern size >", func(t *testing.T) {
		match.MinFileSize = 0
		match.MaxFileSize = 0
		match.CELPattern = "size > 500"
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !matched {
			t.Error("expected to match CEL pattern")
		}
	})

	t.Run("CEL pattern no match", func(t *testing.T) {
		match.CELPattern = "size > 10000"
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if matched {
			t.Error("expected not to match CEL pattern")
		}
	})

	t.Run("CEL pattern invalid", func(t *testing.T) {
		match.CELPattern = "invalid syntax !@#$%"
		_, err := EntryMatch(ctx, entry, match)
		if err == nil {
			t.Error("expected error for invalid CEL pattern")
		}
	})

	// Combined conditions
	t.Run("combined conditions match", func(t *testing.T) {
		match.FileNamePattern = "*.txt"
		match.MinFileSize = 500
		match.CELPattern = "size < 2000"
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !matched {
			t.Error("expected to match all conditions")
		}
	})

	t.Run("combined conditions no match", func(t *testing.T) {
		match.FileNamePattern = "*.pdf"
		match.MinFileSize = 500
		matched, err := EntryMatch(ctx, entry, match)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if matched {
			t.Error("expected not to match when any condition fails")
		}
	})
}

func TestEvalCEL(t *testing.T) {
	var (
		ctx   = context.Background()
		entry = &types.Entry{
			ID:         100,
			Name:       "test.txt",
			Kind:       types.TextKind,
			Size:       5000,
			IsGroup:    false,
			Namespace:  "default",
			CreatedAt:  time.Now(),
			ChangedAt:  time.Now(),
			ModifiedAt: time.Now(),
			AccessAt:   time.Now(),
		}
	)

	tests := []struct {
		name    string
		pattern string
		want    bool
		wantErr bool
	}{
		{"id > 50", "id > 50", true, false},
		{"id < 10", "id < 10", false, false},
		{"name == test.txt", `name == "test.txt"`, true, false},
		{"is_group == false", "is_group == false", true, false},
		{"size > 1000", "size > 1000", true, false},
		{"size < 1000", "size < 1000", false, false},
		{"combined &&", "id > 50 && size > 1000", true, false},
		{"combined ||", "id < 10 || size > 1000", true, false},
		{"not", "!is_group", true, false},
		{"now > 0", "now() > 0", true, false},
		{"time comparison", "now() - created_at < 365 * 24 * 60 * 60", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvalCEL(ctx, entry, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvalCEL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("EvalCEL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildCELFilterFromMatch(t *testing.T) {
	tests := []struct {
		name   string
		match  *types.WorkflowEntryMatch
		want   string
	}{
		{
			name:   "empty match",
			match:  &types.WorkflowEntryMatch{},
			want:   "",
		},
		{
			name: "file types returns empty",
			match: &types.WorkflowEntryMatch{
				FileTypes: "txt",
			},
			want: "",
		},
		{
			name: "parent ID returns empty",
			match: &types.WorkflowEntryMatch{
				ParentID: 100,
			},
			want: "",
		},
		{
			name: "file name pattern",
			match: &types.WorkflowEntryMatch{
				FileNamePattern: "*.txt",
			},
			want: "name LIKE '%.txt'",
		},
		{
			name: "size range",
			match: &types.WorkflowEntryMatch{
				MinFileSize: 100,
				MaxFileSize: 1000,
			},
			want: "size >= 100 && size <= 1000",
		},
		{
			name: "combined conditions",
			match: &types.WorkflowEntryMatch{
				FileNamePattern: "*.txt",
				MinFileSize:     100,
				CELPattern:      "size < 2000",
			},
			want: "name LIKE '%.txt' && size >= 100 && size < 2000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCELFilterFromMatch(tt.match)
			if got != tt.want {
				t.Errorf("BuildCELFilterFromMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobToSQLLike(t *testing.T) {
	tests := []struct {
		pattern string
		want    string
	}{
		{"*.txt", "name LIKE '%.txt'"},
		{"test?.txt", "name LIKE 'test_.txt'"},
		{"*test*.txt", "name LIKE '%test%.txt'"},
		{"exact.txt", "name LIKE 'exact.txt'"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := globToSQLLike(tt.pattern)
			if got != tt.want {
				t.Errorf("globToSQLLike(%v) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}
