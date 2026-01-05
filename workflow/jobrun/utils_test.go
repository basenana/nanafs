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

package jobrun

import (
	"testing"

	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/onsi/gomega"
)

type mockResults struct {
	data map[string]any
}

func (m *mockResults) Set(key string, val any) error {
	m.data[key] = val
	return nil
}

func (m *mockResults) Data() map[string]any {
	return m.data
}

func TestRenderMatrixData(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("single array variable", func(t *testing.T) {
		// Simulate data from previous node using Set method
		mockCtx := &mockResults{data: map[string]any{}}
		paths := []any{"/path/to/file1.webarchive", "/path/to/file2.webarchive", "/path/to/file3.webarchive"}
		err := mockCtx.Set("file_paths", paths)
		g.Expect(err).To(gomega.BeNil())

		matrixData := map[string]string{
			"file_path": "{{ file_paths }}",
		}

		iterations, err := renderMatrixData(matrixData, mockCtx)
		g.Expect(err).To(gomega.BeNil())
		g.Expect(iterations).To(gomega.HaveLen(3))

		g.Expect(iterations[0].Variables["file_path"]).To(gomega.Equal("/path/to/file1.webarchive"))
		g.Expect(iterations[1].Variables["file_path"]).To(gomega.Equal("/path/to/file2.webarchive"))
		g.Expect(iterations[2].Variables["file_path"]).To(gomega.Equal("/path/to/file3.webarchive"))
	})

	t.Run("multiple array variables - cartesian product", func(t *testing.T) {
		// Simulate data from previous node using Set method
		mockCtx := &mockResults{data: map[string]any{}}
		files := []any{"/path/to/file1", "/path/to/file2"}
		docs := []any{"doc1", "doc2", "doc3"}
		err := mockCtx.Set("files", files)
		g.Expect(err).To(gomega.BeNil())
		err = mockCtx.Set("docs", docs)
		g.Expect(err).To(gomega.BeNil())

		matrixData := map[string]string{
			"file":     "{{ files }}",
			"document": "{{ docs }}",
		}

		iterations, err := renderMatrixData(matrixData, mockCtx)
		g.Expect(err).To(gomega.BeNil())
		// Cartesian product: 2 files * 3 docs = 6 iterations
		g.Expect(iterations).To(gomega.HaveLen(6))
	})

	t.Run("empty matrix data", func(t *testing.T) {
		mockCtx := &mockResults{data: map[string]any{}}
		_, err := renderMatrixData(map[string]string{}, mockCtx)
		g.Expect(err).ToNot(gomega.BeNil())
		g.Expect(err.Error()).To(gomega.ContainSubstring("matrix data is empty"))
	})

	t.Run("no array variables", func(t *testing.T) {
		mockCtx := &mockResults{data: map[string]any{}}
		err := mockCtx.Set("single_value", "not_an_array")
		g.Expect(err).To(gomega.BeNil())

		matrixData := map[string]string{
			"var": "{{ single_value }}",
		}

		_, err = renderMatrixData(matrixData, mockCtx)
		g.Expect(err).ToNot(gomega.BeNil())
		g.Expect(err.Error()).To(gomega.ContainSubstring("no array variables found"))
	})

	t.Run("template not resolved", func(t *testing.T) {
		mockCtx := &mockResults{data: map[string]any{}}
		err := mockCtx.Set("other_var", "value")
		g.Expect(err).To(gomega.BeNil())

		matrixData := map[string]string{
			"var": "{{ undefined_var }}",
		}

		_, err = renderMatrixData(matrixData, mockCtx)
		g.Expect(err).ToNot(gomega.BeNil())
		g.Expect(err.Error()).To(gomega.ContainSubstring("no array variables found"))
	})

	t.Run("nested array in object", func(t *testing.T) {
		// Simulate data from docload plugin which returns an object with arrays
		mockCtx := &mockResults{data: map[string]any{}}
		docResults := []any{
			map[string]any{"title": "Doc 1", "path": "/doc1.txt"},
			map[string]any{"title": "Doc 2", "path": "/doc2.txt"},
		}
		err := mockCtx.Set("matrix_results", docResults)
		g.Expect(err).To(gomega.BeNil())

		matrixData := map[string]string{
			"doc": "{{ matrix_results }}",
		}

		iterations, err := renderMatrixData(matrixData, mockCtx)
		g.Expect(err).To(gomega.BeNil())
		g.Expect(iterations).To(gomega.HaveLen(2))
	})
}

func TestCopyMap(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	original := map[string]any{
		"key1": "value1",
		"key2": 123,
		"key3": []any{"a", "b"},
	}

	copied := copyMap(original)

	// Verify values are copied
	g.Expect(copied["key1"]).To(gomega.Equal("value1"))
	g.Expect(copied["key2"]).To(gomega.Equal(123))
	g.Expect(copied["key3"]).To(gomega.Equal([]any{"a", "b"}))

	// Verify it's a different map
	copied["key1"] = "modified"
	g.Expect(original["key1"]).To(gomega.Equal("value1"))
	g.Expect(copied["key1"]).To(gomega.Equal("modified"))
}

var _ pluginapi.Results = &mockResults{}

func TestRenderParams(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("no template placeholders", func(t *testing.T) {
		result := renderParams("plain text without templates", map[string]any{})
		g.Expect(result).To(gomega.Equal("plain text without templates"))
	})

	t.Run("single variable replacement", func(t *testing.T) {
		// Go template requires "index" function for map access
		data := map[string]any{
			"file_path": "/path/to/file.txt",
		}
		// Using index function to access map values
		result := renderParams("{{ index . \"file_path\" }}", data)
		g.Expect(result).To(gomega.Equal("/path/to/file.txt"))
	})

	t.Run("multiple variables", func(t *testing.T) {
		data := map[string]any{
			"base_path": "/data",
			"file_name": "test.txt",
		}
		result := renderParams("{{ index . \"base_path\" }}/{{ index . \"file_name\" }}", data)
		g.Expect(result).To(gomega.Equal("/data/test.txt"))
	})

	t.Run("variable with surrounding text", func(t *testing.T) {
		data := map[string]any{
			"status": "success",
		}
		result := renderParams("Operation completed with status: {{ index . \"status\" }}", data)
		g.Expect(result).To(gomega.Equal("Operation completed with status: success"))
	})

	t.Run("missing variable returns placeholder", func(t *testing.T) {
		data := map[string]any{
			"other_var": "value",
		}
		// When key doesn't exist, index returns "<no value>" in Go templates
		result := renderParams("{{ index . \"missing_var\" }}", data)
		g.Expect(result).To(gomega.Equal("<no value>"))
	})

	t.Run("empty data map", func(t *testing.T) {
		result := renderParams("{{ index . \"var\" }}", map[string]any{})
		g.Expect(result).To(gomega.Equal("<no value>"))
	})

	t.Run("numeric value", func(t *testing.T) {
		data := map[string]any{
			"count": 42,
		}
		result := renderParams("count: {{ index . \"count\" }}", data)
		g.Expect(result).To(gomega.Equal("count: 42"))
	})

	t.Run("multiple occurrences of same variable", func(t *testing.T) {
		data := map[string]any{
			"token": "abc123",
		}
		result := renderParams("token={{ index . \"token\" }}&token={{ index . \"token\" }}", data)
		g.Expect(result).To(gomega.Equal("token=abc123&token=abc123"))
	})
}

func TestRenderMatrixParam(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("simple variable reference", func(t *testing.T) {
		data := map[string]any{
			"file_paths": `["/path1", "/path2"]`,
		}
		result := renderMatrixParam("{{ file_paths }}", data)
		g.Expect(result).To(gomega.Equal(`["/path1", "/path2"]`))
	})

	t.Run("variable with surrounding whitespace", func(t *testing.T) {
		data := map[string]any{
			"value": "test",
		}
		result := renderMatrixParam("{{  value  }}", data)
		g.Expect(result).To(gomega.Equal("test"))
	})

	t.Run("undefined variable returns original", func(t *testing.T) {
		data := map[string]any{
			"other": "value",
		}
		result := renderMatrixParam("{{ undefined }}", data)
		g.Expect(result).To(gomega.Equal("{{ undefined }}"))
	})

	t.Run("non-template string passes through", func(t *testing.T) {
		data := map[string]any{
			"var": "value",
		}
		result := renderMatrixParam("constant text", data)
		g.Expect(result).To(gomega.Equal("constant text"))
	})

	t.Run("simple variable with numeric value", func(t *testing.T) {
		data := map[string]any{
			"count": 42,
		}
		result := renderMatrixParam("{{ count }}", data)
		g.Expect(result).To(gomega.Equal("42"))
	})

	t.Run("nested map value", func(t *testing.T) {
		data := map[string]any{
			"nested": map[string]any{
				"key": "nested_value",
			},
		}
		result := renderMatrixParam("{{ nested }}", data)
		// Simple reference returns the string representation of the value
		g.Expect(result).To(gomega.ContainSubstring("nested_value"))
	})
}
