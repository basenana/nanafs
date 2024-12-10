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
	"github.com/basenana/nanafs/utils"
)

var _ = Describe("testDocumentManage", func() {
	var (
		docId int64
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
			docId = entry.ID
			doc := &types.Document{
				EntryId:       entry.ID,
				Name:          entry.Name,
				ParentEntryID: entry.ParentID,
			}
			err := docManager.SaveDocument(context.TODO(), doc)
			Expect(err).Should(BeNil())
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
				EntryId: entry.ID,
				Unread:  utils.ToPtr(false),
			})
			Expect(err).Should(BeNil())
			doc, err := docManager.GetDocument(context.TODO(), docId)
			Expect(err).Should(BeNil())
			Expect(doc.Unread).Should(Equal(utils.ToPtr(false)))
		})
		It("save should be succeed", func() {
			content := "this is content"
			summary := "this is summary"
			t := true
			newDoc := &types.Document{
				EntryId: int64(123),
				Name:    "test_create_doc",
				Source:  "",
				Unread:  &t,
				Content: content,
				Summary: summary,
			}
			err := docManager.SaveDocument(context.TODO(), newDoc)
			Expect(err).Should(BeNil())

			doc, err := docManager.GetDocument(context.TODO(), int64(123))
			Expect(err).Should(BeNil())
			Expect(doc.Content).Should(Equal(content))
			Expect(doc.Summary).Should(Equal(summary))
			Expect(*doc.Unread).Should(BeTrue())
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
	Context("test list document groups", func() {
		var (
			grp *types.Metadata
		)
		It("create group and document should succeed", func() {
			grp, err = entryMgr.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name:   "test_list_grp",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())

			f := false
			err := docManager.SaveDocument(context.TODO(), &types.Document{
				Name:          "test_list_grp_doc1",
				ParentEntryID: grp.ID,
				Unread:        &f,
			})
			Expect(err).Should(BeNil())

			groups, err := docManager.ListDocumentGroups(context.TODO(), grp.ID, types.DocFilter{
				Unread: &f,
			})
			Expect(err).Should(BeNil())
			Expect(len(groups)).Should(Equal(1))
			Expect(groups[0].ID).Should(Equal(grp.ID))
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
				EntryId:       grp1File1.ID,
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
				Name:          grp1File2.Name,
				EntryId:       grp1File2.ID,
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
				Name:          grp1File3.Name,
				EntryId:       grp1File3.ID,
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
				Name:          grp1File4.Name,
				EntryId:       grp1File4.ID,
				ParentEntryID: grp1File4.ParentID,
			})
			Expect(err).Should(BeNil())
		})
	})
	Context("test handle action", func() {
		It("destroy should be succeed", func() {
			err := docManager.handleEntryEvent(&types.Event{
				Type: events.ActionTypeDestroy,
				Data: types.EventData{
					ID:       grp1File1.ID,
					ParentID: grp1.ID,
				},
				Namespace: types.GlobalNamespaceValue,
			})
			Expect(err).Should(BeNil())
			_, err = docManager.GetDocumentByEntryId(context.TODO(), grp1File1.ID)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
		It("change parent should be succeed", func() {
			err := entryMgr.ChangeEntryParent(context.TODO(), grp1File2.ID, nil, grp1.ID, root.ID, grp1File2.Name, types.ChangeParentAttr{})
			Expect(err).Should(BeNil())

			err = docManager.handleEntryEvent(&types.Event{
				Type: events.ActionTypeChangeParent,
				Data: types.EventData{
					ID:       grp1File2.ID,
					ParentID: root.ID,
				},
				Namespace: types.GlobalNamespaceValue,
			})
			Expect(err).Should(BeNil())
			doc, err := docManager.GetDocumentByEntryId(context.TODO(), grp1File2.ID)
			Expect(err).Should(BeNil())
			Expect(doc.ParentEntryID).Should(Equal(root.ID))
		})
		It("update name should be succeed", func() {
			err := entryMgr.ChangeEntryParent(context.TODO(), grp1File3.ID, nil, grp1.ID, grp1.ID, "test3", types.ChangeParentAttr{})
			Expect(err).Should(BeNil())

			err = docManager.handleEntryEvent(&types.Event{
				Type: events.ActionTypeUpdate,
				Data: types.EventData{
					ID:       grp1File3.ID,
					ParentID: grp1.ID,
				},
				Namespace: types.GlobalNamespaceValue,
			})
			Expect(err).Should(BeNil())
			doc, err := docManager.GetDocumentByEntryId(context.TODO(), grp1File3.ID)
			Expect(err).Should(BeNil())
			Expect(doc.ParentEntryID).Should(Equal(grp1.ID))
			Expect(doc.Name).Should(Equal("test3"))
		})
		It("compact should be succeed", func() {
			err := docManager.handleEntryEvent(&types.Event{
				Type: events.ActionTypeCompact,
				Data: types.EventData{
					ID:       grp1File4.ID,
					ParentID: grp1.ID,
				},
				Namespace: types.GlobalNamespaceValue,
			})
			Expect(err).Should(BeNil())
			doc, err := docManager.GetDocumentByEntryId(context.TODO(), grp1File4.ID)
			Expect(err).Should(BeNil())
			Expect(doc.ParentEntryID).Should(Equal(grp1.ID))
			Expect(doc.Name).Should(Equal(grp1File4.Name))
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
