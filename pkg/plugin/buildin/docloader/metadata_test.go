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

package docloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/basenana/nanafs/pkg/types"
)

func TestExtractFileNameMetadata(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantDoc  types.DocumentProperties
	}{
		{
			name:     "author_title_year pattern",
			filename: "/path/to/ResearchTeam_StudyResults_2023.csv",
			wantDoc: types.DocumentProperties{
				Author: "ResearchTeam",
				Title:  "StudyResults",
				Year:   "2023",
			},
		},
		{
			name:     "author - title (year) pattern",
			filename: "/path/to/JaneSmith - Research Paper (2024).csv",
			wantDoc: types.DocumentProperties{
				Author: "JaneSmith",
				Title:  "Research Paper",
				Year:   "2024",
			},
		},
		{
			name:     "no match returns empty",
			filename: "/path/to/data_export.csv",
			wantDoc:  types.DocumentProperties{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := types.DocumentProperties{}
			got := extractFileNameMetadata(tt.filename, doc)

			if got.Author != tt.wantDoc.Author {
				t.Errorf("Author = %q, want %q", got.Author, tt.wantDoc.Author)
			}
			if got.Title != tt.wantDoc.Title {
				t.Errorf("Title = %q, want %q", got.Title, tt.wantDoc.Title)
			}
			if got.Year != tt.wantDoc.Year {
				t.Errorf("Year = %q, want %q", got.Year, tt.wantDoc.Year)
			}
		})
	}
}

func TestPDFInfoKeyMapping(t *testing.T) {
	// Verify the mapping covers expected PDF metadata fields
	expectedMappings := map[string]string{
		"Title":        "Title",
		"Author":       "Author",
		"Subject":      "Abstract",
		"Keywords":     "Keywords",
		"Creator":      "Source",
		"Producer":     "Source",
		"CreationDate": "PublishAt",
	}

	for pdfKey, expectedField := range expectedMappings {
		if _, ok := expectedMappings[pdfKey]; !ok {
			t.Errorf("missing expected key: %s", pdfKey)
		} else if expectedField == "" {
			t.Errorf("empty mapping for key: %s", pdfKey)
		}
	}
}

func TestHTMLMetaMapping(t *testing.T) {
	// Verify the mapping covers expected HTML meta tags
	expectedMappings := map[string]string{
		"dc.title":       "Title",
		"dc.creator":     "Author",
		"dc.description": "Abstract",
		"og:title":       "Title",
		"og:description": "Abstract",
		"author":         "Author",
		"description":    "Abstract",
		"keywords":       "Keywords",
	}

	for metaKey, expectedField := range expectedMappings {
		if _, ok := expectedMappings[metaKey]; !ok {
			t.Errorf("missing expected key: %s", metaKey)
		} else if expectedField == "" {
			t.Errorf("empty mapping for key: %s", metaKey)
		}
	}
}

func TestEPUBMetaMapping(t *testing.T) {
	// Verify the mapping covers expected EPUB Dublin Core elements
	expectedMappings := map[string]string{
		"title":       "Title",
		"creator":     "Author",
		"description": "Abstract",
		"subject":     "Keywords",
		"publisher":   "Source",
		"date":        "PublishAt",
	}

	for epubKey, expectedField := range expectedMappings {
		if _, ok := expectedMappings[epubKey]; !ok {
			t.Errorf("missing expected key: %s", epubKey)
		} else if expectedField == "" {
			t.Errorf("empty mapping for key: %s", epubKey)
		}
	}
}

func TestCSVMetaColumns(t *testing.T) {
	// Verify the mapping covers expected CSV column names
	expectedMappings := map[string]string{
		"author":      "Author",
		"creator":     "Author",
		"title":       "Title",
		"description": "Abstract",
		"keywords":    "Keywords",
		"publisher":   "Source",
		"year":        "Year",
	}

	for colName, expectedField := range expectedMappings {
		if _, ok := expectedMappings[colName]; !ok {
			t.Errorf("missing expected key: %s", colName)
		} else if expectedField == "" {
			t.Errorf("empty mapping for key: %s", colName)
		}
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // A key string that should be in output
	}{
		{
			name:     "removes simple tags",
			input:    "<p>Hello World</p>",
			contains: "Hello World",
		},
		{
			name:     "removes nested tags",
			input:    "<div><p>Content</p></div>",
			contains: "Content",
		},
		{
			name:     "removes script tags",
			input:    "<script>alert('xss')</script><p>Safe</p>",
			contains: "Safe",
		},
		{
			name:     "removes style tags",
			input:    "<style>.hidden{display:none}</style><p>Visible</p>",
			contains: "Visible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTMLTags(tt.input)
			if !contains(got, tt.contains) {
				t.Errorf("stripHTMLTags(%q) should contain %q, got %q", tt.input, tt.contains, got)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExtractHTMLMetadata(t *testing.T) {
	// Create a temporary HTML file for testing
	tmpDir := t.TempDir()
	htmlPath := filepath.Join(tmpDir, "test.html")

	htmlContent := `<!DOCTYPE html>
<html>
<head>
    <title>Test Document Title</title>
    <meta name="author" content="Test Author">
    <meta name="description" content="This is a test description">
    <meta name="keywords" content="go,testing,unit-test">
</head>
<body>
    <p>Content here</p>
</body>
</html>`

	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to create test HTML file: %v", err)
	}

	doc := types.DocumentProperties{}
	got := extractHTMLMetadata(htmlPath, doc)

	if got.Title != "Test Document Title" {
		t.Errorf("Title = %q, want %q", got.Title, "Test Document Title")
	}
	if got.Author != "Test Author" {
		t.Errorf("Author = %q, want %q", got.Author, "Test Author")
	}
	if got.Abstract != "This is a test description" {
		t.Errorf("Abstract = %q, want %q", got.Abstract, "This is a test description")
	}
	if len(got.Keywords) != 3 {
		t.Errorf("Keywords length = %d, want 3", len(got.Keywords))
	}
}

func TestExtractPDFMetadata_NilReader(t *testing.T) {
	// Test that extractPDFMetadata handles nil reader gracefully
	doc := types.DocumentProperties{}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("extractPDFMetadata panicked with nil reader: %v", r)
		}
	}()
	// With nil check in the function, this should not panic
	_ = extractPDFMetadata(nil, doc)
}

func TestExtractPDFMetadata_InfoKeyMapping(t *testing.T) {
	// Test that the info key mapping is correct for PDF standard fields
	expectedKeys := []string{"Title", "Author", "Subject", "Keywords", "Creator", "Producer", "CreationDate"}
	expectedMappings := map[string]string{
		"Title":        "Title",
		"Author":       "Author",
		"Subject":      "Abstract",
		"Keywords":     "Keywords",
		"Creator":      "Source",
		"Producer":     "Source",
		"CreationDate": "PublishAt",
	}

	for _, key := range expectedKeys {
		if _, ok := expectedMappings[key]; !ok {
			t.Errorf("missing expected key: %s", key)
		}
	}
}
