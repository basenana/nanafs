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

package document

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("testDocumentManage", func() {
	var docId string
	Context("create document", func() {
		It("create should be succeed", func() {
			doc := &types.Document{
				Name: "test_create_doc",
			}
			err := docManager.CreateDocument(context.TODO(), doc)
			Expect(err).Should(BeNil())
			docId = doc.ID
		})
		It("query should be succeed", func() {
			doc, err := docManager.GetDocument(context.TODO(), docId)
			Expect(err).Should(BeNil())
			Expect(doc.Name).Should(Equal("test_create_doc"))
		})
	})
	Context("update document", func() {
		It("update should be succeed", func() {
			err := docManager.UpdateDocument(context.TODO(), &types.Document{
				ID:      docId,
				Name:    "test_create_doc",
				Content: "abc",
			})
			Expect(err).Should(BeNil())
			doc, err := docManager.GetDocument(context.TODO(), docId)
			Expect(err).Should(BeNil())
			Expect(doc.Content).Should(Equal("abc"))
		})
	})
	Context("delete document", func() {
		It("delete should be succeed", func() {
			err := docManager.DeleteDocument(context.TODO(), docId)
			Expect(err).Should(BeNil())
			doc, err := docManager.GetDocument(context.TODO(), docId)
			Expect(err).ShouldNot(BeNil())
			Expect(doc).Should(BeNil())
		})
	})
})
