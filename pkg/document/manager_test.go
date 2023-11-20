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
	var (
		docId string
		entry *types.Metadata
		err   error
	)
	Context("document", func() {
		It("create should be succeed", func() {
			entry, err = entryMgr.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name: "test_doc",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())
			doc := &types.Document{
				Name:          entry.Name,
				ParentEntryID: entry.ParentID,
				OID:           entry.ID,
			}
			err := docManager.SaveDocument(context.TODO(), doc)
			Expect(err).Should(BeNil())
			docId = doc.ID
		})
		It("query should be succeed", func() {
			doc, err := docManager.GetDocument(context.TODO(), docId)
			Expect(err).Should(BeNil())
			Expect(doc.Name).Should(Equal(entry.Name))
		})
		It("get by entry id should be succeed", func() {
			doc, err := docManager.GetDocumentByEntryId(context.TODO(), entry.ID)
			Expect(err).Should(BeNil())
			Expect(doc.Name).Should(Equal(entry.Name))
		})
		It("list should be succeed", func() {
			docs, err := docManager.ListDocuments(context.TODO())
			Expect(err).Should(BeNil())
			Expect(len(docs)).Should(Equal(1))
		})
		It("update should be succeed", func() {
			err := docManager.SaveDocument(context.TODO(), &types.Document{
				ID:      docId,
				Name:    entry.Name,
				Content: "abc",
			})
			Expect(err).Should(BeNil())
			doc, err := docManager.GetDocument(context.TODO(), docId)
			Expect(err).Should(BeNil())
			Expect(doc.Content).Should(Equal("abc"))
		})
		It("save should be succeed", func() {
			keywords := []string{"a", "b"}
			content := "this is content"
			summary := "this is summary"
			newDoc := &types.Document{
				Name:     "test_create_doc",
				OID:      entry.ID,
				Source:   "",
				KeyWords: keywords,
				Content:  content,
				Summary:  summary,
			}
			err := docManager.SaveDocument(context.TODO(), newDoc)
			Expect(err).Should(BeNil())

			doc, err := docManager.GetDocument(context.TODO(), docId)
			Expect(err).Should(BeNil())
			Expect(doc.Content).Should(Equal(content))
			Expect(doc.KeyWords).Should(Equal(keywords))
			Expect(doc.Summary).Should(Equal(summary))
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
