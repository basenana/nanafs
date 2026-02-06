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
})

var _ = Describe("FileBasedResults", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "jobrun-test-*")
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	It("should create new instance", func() {
		filePath := filepath.Join(tempDir, "results.gob")
		results, err := NewFileBasedResults(filePath)
		Expect(err).To(BeNil())
		Expect(results).ToNot(BeNil())
	})

	It("should create file on first Set", func() {
		filePath := filepath.Join(tempDir, "results.gob")
		results, err := NewFileBasedResults(filePath)
		Expect(err).To(BeNil())

		err = results.Set("key1", "value1")
		Expect(err).To(BeNil())

		_, err = os.Stat(filePath)
		Expect(err).To(BeNil())
	})

	It("should handle string values", func() {
		filePath := filepath.Join(tempDir, "results.gob")
		results, err := NewFileBasedResults(filePath)
		Expect(err).To(BeNil())

		err = results.Set("test_key", "test_value")
		Expect(err).To(BeNil())

		// Verify data was set
		data := results.Data()
		Expect(data).To(HaveKey("test_key"))
	})
})

var _ = Describe("ResultFilePath", func() {
	It("should return correct path for base path", func() {
		basePath := "/tmp/workflow"
		resultPath := ResultFilePath(basePath)
		Expect(resultPath).To(Equal(filepath.Join(basePath, ".workflowcontext.gob")))
	})
})
