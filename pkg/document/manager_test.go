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

	"github.com/basenana/nanafs/pkg/events"
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

var _ = Describe("TestHandleEvent", func() {
	var (
		grp1      *types.Metadata
		grp1File1 *types.Metadata
		grp1File2 *types.Metadata
		grp1File3 *types.Metadata
		grp1File4 *types.Metadata
	)

	Context("create grp and files", func() {
		It("create grp and files should be succeed", func() {
			var err error
			grp1, err = entryMgr.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name:   "test_doc_grp1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			grp1File1, err = entryMgr.CreateEntry(context.TODO(), grp1.ID, types.EntryAttr{
				Name:   "test_doc_grp1_file1",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			err = docManager.SaveDocument(context.TODO(), &types.Document{
				OID:           grp1File1.ID,
				Name:          grp1File1.Name,
				ParentEntryID: grp1File1.ParentID,
			})
			Expect(err).Should(BeNil())

			grp1File2, err = entryMgr.CreateEntry(context.TODO(), grp1.ID, types.EntryAttr{
				Name:   "test_doc_grp1_file2",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			err = docManager.SaveDocument(context.TODO(), &types.Document{
				OID:           grp1File2.ID,
				Name:          grp1File2.Name,
				ParentEntryID: grp1File2.ParentID,
			})
			Expect(err).Should(BeNil())

			grp1File3, err = entryMgr.CreateEntry(context.TODO(), grp1.ID, types.EntryAttr{
				Name:   "test_doc_grp1_file3",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			err = docManager.SaveDocument(context.TODO(), &types.Document{
				OID:           grp1File3.ID,
				Name:          grp1File3.Name,
				ParentEntryID: grp1File3.ParentID,
			})
			Expect(err).Should(BeNil())

			grp1File4, err = entryMgr.CreateEntry(context.TODO(), grp1.ID, types.EntryAttr{
				Name:   "test_doc_grp1_file4",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			err = docManager.SaveDocument(context.TODO(), &types.Document{
				OID:           grp1File4.ID,
				Name:          grp1File4.Name,
				ParentEntryID: grp1File4.ParentID,
			})
			Expect(err).Should(BeNil())
		})
	})
	Context("test handle action", func() {
		It("destroy should be succeed", func() {
			err := docManager.handleEvent(&types.EntryEvent{
				Type: events.ActionTypeDestroy,
				Data: types.EventData{
					ID:       grp1File1.ID,
					ParentID: grp1.ID,
				},
			})
			Expect(err).Should(BeNil())
			_, err = docManager.GetDocumentByEntryId(context.TODO(), grp1File1.ID)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
		It("change parent should be succeed", func() {
			err := entryMgr.ChangeEntryParent(context.TODO(), grp1File2.ID, nil, grp1.ID, root.ID, grp1File2.Name, types.ChangeParentAttr{})
			Expect(err).Should(BeNil())

			err = docManager.handleEvent(&types.EntryEvent{
				Type: events.ActionTypeChangeParent,
				Data: types.EventData{
					ID:       grp1File2.ID,
					ParentID: root.ID,
				},
			})
			Expect(err).Should(BeNil())
			doc, err := docManager.GetDocumentByEntryId(context.TODO(), grp1File2.ID)
			Expect(err).Should(BeNil())
			Expect(doc.ParentEntryID).Should(Equal(root.ID))
		})
		It("update name should be succeed", func() {
			err := entryMgr.ChangeEntryParent(context.TODO(), grp1File3.ID, nil, grp1.ID, grp1.ID, "test3", types.ChangeParentAttr{})
			Expect(err).Should(BeNil())

			err = docManager.handleEvent(&types.EntryEvent{
				Type: events.ActionTypeUpdate,
				Data: types.EventData{
					ID:       grp1File3.ID,
					ParentID: grp1.ID,
				},
			})
			Expect(err).Should(BeNil())
			doc, err := docManager.GetDocumentByEntryId(context.TODO(), grp1File3.ID)
			Expect(err).Should(BeNil())
			Expect(doc.ParentEntryID).Should(Equal(grp1.ID))
			Expect(doc.Name).Should(Equal("test3"))
		})
		It("compact should be succeed", func() {
			err := docManager.handleEvent(&types.EntryEvent{
				Type: events.ActionTypeCompact,
				Data: types.EventData{
					ID:       grp1File4.ID,
					ParentID: grp1.ID,
				},
			})
			Expect(err).Should(BeNil())
			doc, err := docManager.GetDocumentByEntryId(context.TODO(), grp1File4.ID)
			Expect(err).Should(BeNil())
			Expect(doc.ParentEntryID).Should(Equal(grp1.ID))
			Expect(doc.Name).Should(Equal(grp1File4.Name))
			Expect(doc.Desync).Should(BeTrue())
		})
	})
})

var accessPermissions = &types.Access{
	Permissions: []types.Permission{
		types.PermOwnerRead,
		types.PermOwnerWrite,
		types.PermOwnerExec,
		types.PermGroupRead,
		types.PermGroupWrite,
		types.PermOthersRead,
	},
}
