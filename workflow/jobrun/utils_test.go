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
		mockCtx := &mockResults{data: map[string]any{}}
		paths := []any{"/path/to/file1.webarchive", "/path/to/file2.webarchive", "/path/to/file3.webarchive"}
		err := mockCtx.Set("file_paths", paths)
		g.Expect(err).To(gomega.BeNil())

		matrixData := map[string]any{
			"file_path": "$.file_paths",
		}

		iterations, err := renderMatrixData(matrixData, mockCtx)
		g.Expect(err).To(gomega.BeNil())
		g.Expect(iterations).To(gomega.HaveLen(3))

		g.Expect(iterations[0].Variables["file_path"]).To(gomega.Equal("/path/to/file1.webarchive"))
		g.Expect(iterations[1].Variables["file_path"]).To(gomega.Equal("/path/to/file2.webarchive"))
		g.Expect(iterations[2].Variables["file_path"]).To(gomega.Equal("/path/to/file3.webarchive"))
	})

	t.Run("multiple array variables - cartesian product", func(t *testing.T) {
		mockCtx := &mockResults{data: map[string]any{}}
		files := []any{"/path/to/file1", "/path/to/file2"}
		docs := []any{"doc1", "doc2", "doc3"}
		err := mockCtx.Set("files", files)
		g.Expect(err).To(gomega.BeNil())
		err = mockCtx.Set("docs", docs)
		g.Expect(err).To(gomega.BeNil())

		matrixData := map[string]any{
			"file":     "$.files",
			"document": "$.docs",
		}

		iterations, err := renderMatrixData(matrixData, mockCtx)
		g.Expect(err).To(gomega.BeNil())
		g.Expect(iterations).To(gomega.HaveLen(6))
	})

	t.Run("empty matrix data", func(t *testing.T) {
		mockCtx := &mockResults{data: map[string]any{}}
		_, err := renderMatrixData(map[string]any{}, mockCtx)
		g.Expect(err).ToNot(gomega.BeNil())
		g.Expect(err.Error()).To(gomega.ContainSubstring("matrix data is empty"))
	})

	t.Run("no array variables", func(t *testing.T) {
		mockCtx := &mockResults{data: map[string]any{}}
		err := mockCtx.Set("single_value", "not_an_array")
		g.Expect(err).To(gomega.BeNil())

		matrixData := map[string]any{
			"var": "$.single_value",
		}

		_, err = renderMatrixData(matrixData, mockCtx)
		g.Expect(err).ToNot(gomega.BeNil())
		g.Expect(err.Error()).To(gomega.ContainSubstring("no array variables found"))
	})

	t.Run("template not resolved", func(t *testing.T) {
		mockCtx := &mockResults{data: map[string]any{}}
		err := mockCtx.Set("other_var", "value")
		g.Expect(err).To(gomega.BeNil())

		matrixData := map[string]any{
			"var": "$.undefined_var",
		}

		_, err = renderMatrixData(matrixData, mockCtx)
		g.Expect(err).ToNot(gomega.BeNil())
		g.Expect(err.Error()).To(gomega.ContainSubstring("no array variables found"))
	})

	t.Run("nested array in object", func(t *testing.T) {
		mockCtx := &mockResults{data: map[string]any{}}
		docResults := []any{
			map[string]any{"title": "Doc 1", "path": "/doc1.txt"},
			map[string]any{"title": "Doc 2", "path": "/doc2.txt"},
		}
		err := mockCtx.Set("matrix_results", docResults)
		g.Expect(err).To(gomega.BeNil())

		matrixData := map[string]any{
			"doc": "$.matrix_results",
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

	g.Expect(copied["key1"]).To(gomega.Equal("value1"))
	g.Expect(copied["key2"]).To(gomega.Equal(123))
	g.Expect(copied["key3"]).To(gomega.Equal([]any{"a", "b"}))

	copied["key1"] = "modified"
	g.Expect(original["key1"]).To(gomega.Equal("value1"))
	g.Expect(copied["key1"]).To(gomega.Equal("modified"))
}

var _ Results = &mockResults{}

func TestRenderParams(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("plain text passes through", func(t *testing.T) {
		result := renderParams("plain text without templates", map[string]any{})
		g.Expect(result).To(gomega.Equal("plain text without templates"))
	})

	t.Run("jsonpath variable replacement", func(t *testing.T) {
		data := map[string]any{
			"file_path": "/path/to/file.txt",
		}
		result := renderParams("$.file_path", data)
		g.Expect(result).To(gomega.Equal("/path/to/file.txt"))
	})

	t.Run("jsonpath multiple variables - not supported", func(t *testing.T) {
		// JSONPath only supports single expressions, not string concatenation
		// This test documents that multiple $ expressions return original
		data := map[string]any{
			"base_path": "/data",
			"file_name": "test.txt",
		}
		result := renderParams("$.base_path/$.file_name", data)
		// Multiple JSONPath in one string is not supported - returns original
		g.Expect(result).To(gomega.Equal("$.base_path/$.file_name"))
	})

	t.Run("jsonpath with surrounding text - not supported", func(t *testing.T) {
		// JSONPath with surrounding text is not supported
		// This test documents that text around $ expression returns original
		data := map[string]any{
			"status": "success",
		}
		result := renderParams("Operation completed with status: $.status", data)
		g.Expect(result).To(gomega.Equal("Operation completed with status: $.status"))
	})

	t.Run("jsonpath missing variable returns original", func(t *testing.T) {
		data := map[string]any{
			"other_var": "value",
		}
		result := renderParams("$.missing_var", data)
		g.Expect(result).To(gomega.Equal("$.missing_var"))
	})

	t.Run("non-string value passes through", func(t *testing.T) {
		nested := map[string]any{"key": "value"}
		result := renderParams(nested, map[string]any{})
		g.Expect(result).To(gomega.Equal(nested))
	})

	t.Run("jsonpath numeric value", func(t *testing.T) {
		data := map[string]any{
			"count": 42,
		}
		result := renderParams("$.count", data)
		g.Expect(result).To(gomega.Equal(42))
	})

	t.Run("jsonpath array value", func(t *testing.T) {
		data := map[string]any{
			"items": []any{"a", "b", "c"},
		}
		result := renderParams("$.items", data)
		g.Expect(result).To(gomega.Equal([]any{"a", "b", "c"}))
	})

	t.Run("jsonpath nested object", func(t *testing.T) {
		data := map[string]any{
			"nested": map[string]any{
				"key": "nested_value",
			},
		}
		result := renderParams("$.nested", data)
		g.Expect(result).To(gomega.Equal(map[string]any{"key": "nested_value"}))
	})

	t.Run("jsonpath nested key access", func(t *testing.T) {
		data := map[string]any{
			"nested": map[string]any{
				"key": "nested_value",
			},
		}
		result := renderParams("$.nested.key", data)
		g.Expect(result).To(gomega.Equal("nested_value"))
	})
}

func TestRenderMatrixParam(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("simple jsonpath reference", func(t *testing.T) {
		data := map[string]any{
			"file_paths": []any{"/path1", "/path2"},
		}
		result := renderMatrixParam("$.file_paths", data)
		g.Expect(result).To(gomega.Equal([]any{"/path1", "/path2"}))
	})

	t.Run("undefined jsonpath returns original", func(t *testing.T) {
		data := map[string]any{
			"other": "value",
		}
		result := renderMatrixParam("$.undefined", data)
		g.Expect(result).To(gomega.Equal("$.undefined"))
	})

	t.Run("non-jsonpath string passes through", func(t *testing.T) {
		data := map[string]any{
			"var": "value",
		}
		result := renderMatrixParam("constant text", data)
		g.Expect(result).To(gomega.Equal("constant text"))
	})

	t.Run("jsonpath numeric value", func(t *testing.T) {
		data := map[string]any{
			"count": 42,
		}
		result := renderMatrixParam("$.count", data)
		g.Expect(result).To(gomega.Equal(42))
	})

	t.Run("jsonpath nested map value", func(t *testing.T) {
		data := map[string]any{
			"nested": map[string]any{
				"key": "nested_value",
			},
		}
		result := renderMatrixParam("$.nested", data)
		g.Expect(result).To(gomega.Equal(map[string]any{"key": "nested_value"}))
	})

	t.Run("jsonpath nested key access", func(t *testing.T) {
		data := map[string]any{
			"nested": map[string]any{
				"key": "nested_value",
			},
		}
		result := renderMatrixParam("$.nested.key", data)
		g.Expect(result).To(gomega.Equal("nested_value"))
	})

	t.Run("non-string value passes through", func(t *testing.T) {
		nested := map[string]any{"key": "value"}
		result := renderMatrixParam(nested, map[string]any{})
		g.Expect(result).To(gomega.Equal(nested))
	})
}

func TestGetJSONPathValue(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("root level key", func(t *testing.T) {
		data := map[string]any{
			"file_path": "/path/to/file.txt",
		}
		result, err := getJSONPathValue("$.file_path", data)
		g.Expect(err).To(gomega.BeNil())
		g.Expect(result).To(gomega.Equal("/path/to/file.txt"))
	})

	t.Run("nested key access", func(t *testing.T) {
		data := map[string]any{
			"nested": map[string]any{
				"key": "nested_value",
			},
		}
		result, err := getJSONPathValue("$.nested.key", data)
		g.Expect(err).To(gomega.BeNil())
		g.Expect(result).To(gomega.Equal("nested_value"))
	})

	t.Run("array access", func(t *testing.T) {
		data := map[string]any{
			"items": []any{"first", "second", "third"},
		}
		result, err := getJSONPathValue("$.items[0]", data)
		g.Expect(err).To(gomega.BeNil())
		g.Expect(result).To(gomega.Equal("first"))
	})

	t.Run("path not found", func(t *testing.T) {
		data := map[string]any{
			"key": "value",
		}
		_, err := getJSONPathValue("$.undefined", data)
		g.Expect(err).ToNot(gomega.BeNil())
	})
}
