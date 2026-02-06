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
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MemoryBasedResults", func() {
	It("should create new instance with empty data", func() {
		results := NewMemBasedResults()
		Expect(results.Data()).To(BeEmpty())
	})

	It("should set and retrieve string values", func() {
		results := NewMemBasedResults()
		err := results.Set("key1", "value1")
		Expect(err).To(BeNil())

		data := results.Data()
		Expect(data["key1"]).To(Equal("value1"))
	})

	It("should overwrite existing values", func() {
		results := NewMemBasedResults()
		_ = results.Set("key", "original")
		_ = results.Set("key", "updated")

		data := results.Data()
		Expect(data["key"]).To(Equal("updated"))
	})

	It("should handle nested map values", func() {
		results := NewMemBasedResults()
		nested := map[string]any{"inner": "value"}
		err := results.Set("nested", nested)
		Expect(err).To(BeNil())

		data := results.Data()
		Expect(data["nested"]).ToNot(BeNil())
		nestedMap, ok := data["nested"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(nestedMap["inner"]).To(Equal("value"))
	})

	It("should return copy of data", func() {
		results := NewMemBasedResults()
		_ = results.Set("key", "value")

		data1 := results.Data()
		data1["key"] = "modified"

		data2 := results.Data()
		Expect(data2["key"]).To(Equal("value"))
	})
})

var _ = Describe("JSONFileResults", func() {
	var workdir string

	BeforeEach(func() {
		var err error
		workdir, err = os.MkdirTemp(tempDir, "test-results-")
		Expect(err).To(BeNil())
	})

	It("should create new instance", func() {
		filePath := filepath.Join(workdir, "results.json")
		results, err := NewJSONFileResults(filePath)
		Expect(err).To(BeNil())
		Expect(results).ToNot(BeNil())
	})

	It("should create file on first Set", func() {
		filePath := filepath.Join(workdir, "results.json")
		results, err := NewJSONFileResults(filePath)
		Expect(err).To(BeNil())

		err = results.Set("key1", "value1")
		Expect(err).To(BeNil())

		_, err = os.Stat(filePath)
		Expect(err).To(BeNil())
	})

	It("should persist and reload data", func() {
		filePath := filepath.Join(workdir, "results.json")

		// Create and set data
		results1, err := NewJSONFileResults(filePath)
		Expect(err).To(BeNil())
		_ = results1.Set("key1", "value1")
		_ = results1.Set("nested", map[string]any{"a": 1, "b": "test"})

		// Create new instance and reload
		results2, err := NewJSONFileResults(filePath)
		Expect(err).To(BeNil())

		data := results2.Data()
		Expect(data["key1"]).To(Equal("value1"))
		nested, ok := data["nested"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(nested["a"]).To(Equal(float64(1)))
		Expect(nested["b"]).To(Equal("test"))
	})

	It("should handle string values", func() {
		filePath := filepath.Join(workdir, "results.json")
		results, err := NewJSONFileResults(filePath)
		Expect(err).To(BeNil())

		err = results.Set("test_key", "test_value")
		Expect(err).To(BeNil())

		data := results.Data()
		Expect(data).To(HaveKey("test_key"))
	})

	It("should overwrite existing values", func() {
		filePath := filepath.Join(workdir, "results.json")
		results, err := NewJSONFileResults(filePath)
		Expect(err).To(BeNil())

		_ = results.Set("key", "original")
		_ = results.Set("key", "updated")

		data := results.Data()
		Expect(data["key"]).To(Equal("updated"))
	})

	It("should handle array values", func() {
		filePath := filepath.Join(workdir, "results.json")
		results, err := NewJSONFileResults(filePath)
		Expect(err).To(BeNil())

		arr := []string{"a", "b", "c"}
		err = results.Set("array", arr)
		Expect(err).To(BeNil())

		// Reload and verify
		results2, err := NewJSONFileResults(filePath)
		Expect(err).To(BeNil())
		data := results2.Data()
		Expect(data["array"]).ToNot(BeNil())
	})

	It("should preserve JSON file on reload", func() {
		filePath := filepath.Join(workdir, "results.json")

		results1, err := NewJSONFileResults(filePath)
		Expect(err).To(BeNil())
		_ = results1.Set("key", "value")

		// Verify file content is valid JSON
		content, err := os.ReadFile(filePath)
		Expect(err).To(BeNil())
		var jsonData map[string]any
		err = json.Unmarshal(content, &jsonData)
		Expect(err).To(BeNil())
		Expect(jsonData).To(HaveKeyWithValue("key", "value"))
	})
})
