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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RenderMatrixData", func() {
	Context("with single array variable", func() {
		It("should generate iterations for each array element", func() {
			mockCtx := &mockResults{data: map[string]any{}}
			paths := []any{"/path/to/file1.webarchive", "/path/to/file2.webarchive", "/path/to/file3.webarchive"}
			_ = mockCtx.Set("file_paths", paths)

			matrixData := map[string]any{
				"file_path": "$.file_paths",
			}

			iterations, err := renderMatrixData(matrixData, mockCtx.Data())
			Expect(err).To(BeNil())
			Expect(iterations).To(HaveLen(3))

			Expect(iterations[0].Variables["file_path"]).To(Equal("/path/to/file1.webarchive"))
			Expect(iterations[1].Variables["file_path"]).To(Equal("/path/to/file2.webarchive"))
			Expect(iterations[2].Variables["file_path"]).To(Equal("/path/to/file3.webarchive"))
		})
	})

	Context("with multiple variables of equal length", func() {
		It("should generate Cartesian product of arrays", func() {
			mockCtx := &mockResults{data: map[string]any{}}
			files := []any{"file1", "file2"}
			docs := []any{"doc1", "doc2"}
			_ = mockCtx.Set("files", files)
			_ = mockCtx.Set("docs", docs)

			matrixData := map[string]any{
				"file":     "$.files",
				"document": "$.docs",
			}

			iterations, err := renderMatrixData(matrixData, mockCtx.Data())
			Expect(err).To(BeNil())
			Expect(iterations).To(HaveLen(2))

			Expect(iterations[0].Variables).To(HaveKeyWithValue("file", "file1"))
			Expect(iterations[0].Variables).To(HaveKeyWithValue("document", "doc1"))
			Expect(iterations[1].Variables).To(HaveKeyWithValue("file", "file2"))
			Expect(iterations[1].Variables).To(HaveKeyWithValue("document", "doc2"))
		})
	})

	Context("with multiple variables of different lengths", func() {
		It("should use shorter array length and fill missing values", func() {
			mockCtx := &mockResults{data: map[string]any{}}
			files := []any{"file1", "file2", "file3"}
			docs := []any{"doc1", "doc2"}
			_ = mockCtx.Set("files", files)
			_ = mockCtx.Set("docs", docs)

			matrixData := map[string]any{
				"file":     "$.files",
				"document": "$.docs",
			}

			iterations, err := renderMatrixData(matrixData, mockCtx.Data())
			Expect(err).To(BeNil())
			Expect(iterations).To(HaveLen(3))

			Expect(iterations[0].Variables).To(HaveKeyWithValue("file", "file1"))
			Expect(iterations[0].Variables).To(HaveKeyWithValue("document", "doc1"))

			Expect(iterations[1].Variables).To(HaveKeyWithValue("file", "file2"))
			Expect(iterations[1].Variables).To(HaveKeyWithValue("document", "doc2"))

			Expect(iterations[2].Variables).To(HaveKeyWithValue("file", "file3"))
			Expect(iterations[2].Variables).ToNot(HaveKey("document"))
		})
	})

	Context("with empty array variable", func() {
		It("should return empty iterations", func() {
			mockCtx := &mockResults{data: map[string]any{}}
			_ = mockCtx.Set("empty_var", []any{})

			matrixData := map[string]any{
				"var": "$.empty_var",
			}

			iterations, err := renderMatrixData(matrixData, mockCtx.Data())
			Expect(err).To(BeNil())
			Expect(iterations).To(BeEmpty())
		})
	})

	Context("with empty matrix data", func() {
		It("should return empty iterations", func() {
			mockCtx := &mockResults{data: map[string]any{}}
			iterations, err := renderMatrixData(map[string]any{}, mockCtx.Data())
			Expect(err).To(BeNil())
			Expect(iterations).To(BeEmpty())
		})
	})

	Context("with no array variables", func() {
		It("should return empty iterations when value is not an array", func() {
			mockCtx := &mockResults{data: map[string]any{}}
			_ = mockCtx.Set("single_value", "not_an_array")

			matrixData := map[string]any{
				"var": "$.single_value",
			}

			iterations, err := renderMatrixData(matrixData, mockCtx.Data())
			Expect(err).To(BeNil())
			Expect(iterations).To(BeEmpty())
		})

		It("should return empty iterations when variable is undefined", func() {
			mockCtx := &mockResults{data: map[string]any{}}
			_ = mockCtx.Set("other_var", "value")

			matrixData := map[string]any{
				"var": "$.undefined_var",
			}

			iterations, err := renderMatrixData(matrixData, mockCtx.Data())
			Expect(err).To(BeNil())
			Expect(iterations).To(BeEmpty())
		})
	})

	Context("with nested array in object", func() {
		It("should handle array values from nested structures", func() {
			mockCtx := &mockResults{data: map[string]any{}}
			docResults := []any{
				map[string]any{"title": "Doc 1", "path": "/doc1.txt"},
				map[string]any{"title": "Doc 2", "path": "/doc2.txt"},
			}
			_ = mockCtx.Set("matrix_results", docResults)

			matrixData := map[string]any{
				"doc": "$.matrix_results",
			}

			iterations, err := renderMatrixData(matrixData, mockCtx.Data())
			Expect(err).To(BeNil())
			Expect(iterations).To(HaveLen(2))
		})
	})
})

var _ = Describe("CopyMap", func() {
	It("should create a deep copy of the map", func() {
		original := map[string]any{
			"key1": "value1",
			"key2": 123,
			"key3": []any{"a", "b"},
		}

		copied := copyMap(original)

		Expect(copied["key1"]).To(Equal("value1"))
		Expect(copied["key2"]).To(Equal(123))
		Expect(copied["key3"]).To(Equal([]any{"a", "b"}))
	})

	It("should not modify original when copy is modified", func() {
		original := map[string]any{
			"key1": "value1",
		}

		copied := copyMap(original)
		copied["key1"] = "modified"

		Expect(original["key1"]).To(Equal("value1"))
		Expect(copied["key1"]).To(Equal("modified"))
	})
})

var _ = Describe("RenderParams", func() {
	Context("with plain text", func() {
		It("should pass through without modification", func() {
			result := renderParams("plain text without templates", map[string]any{})
			Expect(result).To(Equal("plain text without templates"))
		})
	})

	Context("with JSONPath variable", func() {
		It("should replace variable with its value", func() {
			data := map[string]any{
				"file_path": "/path/to/file.txt",
			}
			result := renderParams("$.file_path", data)
			Expect(result).To(Equal("/path/to/file.txt"))
		})

		It("should return empty string for missing variable", func() {
			data := map[string]any{
				"other_var": "value",
			}
			result := renderParams("$.missing_var", data)
			Expect(result).To(Equal(""))
		})

		It("should handle numeric values", func() {
			data := map[string]any{
				"count": 42,
			}
			result := renderParams("$.count", data)
			Expect(result).To(Equal(42))
		})

		It("should handle array values", func() {
			data := map[string]any{
				"items": []any{"a", "b", "c"},
			}
			result := renderParams("$.items", data)
			Expect(result).To(Equal([]any{"a", "b", "c"}))
		})

		It("should handle nested object values", func() {
			data := map[string]any{
				"nested": map[string]any{
					"key": "nested_value",
				},
			}
			result := renderParams("$.nested", data)
			Expect(result).To(Equal(map[string]any{"key": "nested_value"}))
		})

		It("should handle nested key access", func() {
			data := map[string]any{
				"nested": map[string]any{
					"key": "nested_value",
				},
			}
			result := renderParams("$.nested.key", data)
			Expect(result).To(Equal("nested_value"))
		})
	})

	Context("with unsupported JSONPath patterns", func() {
		It("should return original string for multiple $ expressions", func() {
			data := map[string]any{
				"base_path": "/data",
				"file_name": "test.txt",
			}
			result := renderParams("$.base_path/$.file_name", data)
			Expect(result).To(Equal("$.base_path/$.file_name"))
		})

		It("should return original string for surrounding text", func() {
			data := map[string]any{
				"status": "success",
			}
			result := renderParams("Operation completed with status: $.status", data)
			Expect(result).To(Equal("Operation completed with status: $.status"))
		})
	})

	Context("with non-string values", func() {
		It("should pass through non-string values unchanged", func() {
			nested := map[string]any{"key": "value"}
			result := renderParams(nested, map[string]any{})
			Expect(result).To(Equal(nested))
		})
	})
})

var _ = Describe("RenderMatrixParam", func() {
	Context("with simple JSONPath reference", func() {
		It("should return array values", func() {
			data := map[string]any{
				"file_paths": []any{"/path1", "/path2"},
			}
			result := renderMatrixParam("$.file_paths", data)
			Expect(result).To(Equal([]any{"/path1", "/path2"}))
		})

		It("should return empty string for undefined variable", func() {
			data := map[string]any{
				"other": "value",
			}
			result := renderMatrixParam("$.undefined", data)
			Expect(result).To(Equal(""))
		})

		It("should handle numeric values", func() {
			data := map[string]any{
				"count": 42,
			}
			result := renderMatrixParam("$.count", data)
			Expect(result).To(Equal(42))
		})

		It("should handle nested map values", func() {
			data := map[string]any{
				"nested": map[string]any{
					"key": "nested_value",
				},
			}
			result := renderMatrixParam("$.nested", data)
			Expect(result).To(Equal(map[string]any{"key": "nested_value"}))
		})

		It("should handle nested key access", func() {
			data := map[string]any{
				"nested": map[string]any{
					"key": "nested_value",
				},
			}
			result := renderMatrixParam("$.nested.key", data)
			Expect(result).To(Equal("nested_value"))
		})
	})

	Context("with non-JSONPath strings", func() {
		It("should pass through constant text unchanged", func() {
			data := map[string]any{
				"var": "value",
			}
			result := renderMatrixParam("constant text", data)
			Expect(result).To(Equal("constant text"))
		})
	})

	Context("with non-string values", func() {
		It("should pass through non-string values unchanged", func() {
			nested := map[string]any{"key": "value"}
			result := renderMatrixParam(nested, map[string]any{})
			Expect(result).To(Equal(nested))
		})
	})
})

var _ = Describe("GetJSONPathValue", func() {
	Context("with root level key", func() {
		It("should return value for existing key", func() {
			data := map[string]any{
				"file_path": "/path/to/file.txt",
			}
			result, err := getJSONPathValue("$.file_path", data)
			Expect(err).To(BeNil())
			Expect(result).To(Equal("/path/to/file.txt"))
		})
	})

	Context("with nested key access", func() {
		It("should return nested value", func() {
			data := map[string]any{
				"nested": map[string]any{
					"key": "nested_value",
				},
			}
			result, err := getJSONPathValue("$.nested.key", data)
			Expect(err).To(BeNil())
			Expect(result).To(Equal("nested_value"))
		})
	})

	Context("with array access", func() {
		It("should return first element", func() {
			data := map[string]any{
				"items": []any{"first", "second", "third"},
			}
			result, err := getJSONPathValue("$.items[0]", data)
			Expect(err).To(BeNil())
			Expect(result).To(Equal("first"))
		})
	})

	Context("with undefined path", func() {
		It("should return empty string", func() {
			data := map[string]any{
				"key": "value",
			}
			result, err := getJSONPathValue("$.undefined", data)
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})
})
