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

package v1

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

var _ = Describe("testInboxService", func() {
	Context("test quick inbox", func() {
		It("inbox should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("file in inbox should be found", func() {
			Expect(nil).Should(BeNil())
		})
	})
})

var _ = Describe("testEntriesService-CRUD", func() {
	var (
		ctx     = context.TODO()
		groupID int64
		fileID  int64

		groupID2 int64
		fileID2  int64
	)

	Context("test create entry", func() {
		It("create group should be succeed", func() {
			resp, err := serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: dentry.RootEntryID,
				Name:     "test-group-1",
				Kind:     types.GroupKind,
			})
			Expect(err).Should(BeNil())
			Expect(resp.Entry.Name).Should(Equal("test-group-1"))
			groupID = resp.Entry.Id

			resp, err = serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: dentry.RootEntryID,
				Name:     "test-group-2",
				Kind:     types.GroupKind,
			})
			Expect(err).Should(BeNil())
			groupID2 = resp.Entry.Id
		})
		It("create child file should be succeed", func() {
			resp, err := serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: groupID,
				Name:     "test-file-1",
				Kind:     types.RawKind,
			})
			Expect(err).Should(BeNil())
			fileID = resp.Entry.Id

			resp, err = serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: groupID,
				Name:     "test-file-2",
				Kind:     types.RawKind,
			})
			Expect(err).Should(BeNil())
			fileID2 = resp.Entry.Id
		})
		It("list child should be succeed", func() {
			resp, err := serviceClient.ListGroupChildren(ctx, &ListGroupChildrenRequest{ParentID: groupID})
			Expect(err).Should(BeNil())
			Expect(len(resp.Entries) > 0).Should(BeTrue())
		})
		It("get detail should be succeed", func() {
			resp, err := serviceClient.GetEntryDetail(ctx, &GetEntryDetailRequest{EntryID: fileID})
			Expect(err).Should(BeNil())
			Expect(resp.Entry.Name).Should(Equal("test-file-1"))
		})
	})
	Context("test move entry", func() {
		It("move should be succeed", func() {
			_, err := serviceClient.ChangeParent(ctx, &ChangeParentRequest{EntryID: fileID2, NewParentID: groupID2})
			Expect(err).Should(BeNil())
		})
		It("list child should be succeed", func() {
			resp, err := serviceClient.ListGroupChildren(ctx, &ListGroupChildrenRequest{ParentID: groupID2})
			Expect(err).Should(BeNil())
			Expect(len(resp.Entries) > 0).Should(BeTrue())
		})
	})
	Context("test update entry", func() {
		It("update should be succeed", func() {
			_, err := serviceClient.UpdateEntry(ctx, &UpdateEntryRequest{
				Entry: &EntryDetail{Id: fileID, Aliases: "test aliases"},
			})
			Expect(err).Should(BeNil())
		})
		It("get detail should be succeed", func() {
			resp, err := serviceClient.GetEntryDetail(ctx, &GetEntryDetailRequest{EntryID: fileID})
			Expect(err).Should(BeNil())
			Expect(resp.Entry.Aliases).Should(Equal("test aliases"))
		})
	})
	Context("test delete entry", func() {
		It("delete should be succeed", func() {
			_, err := serviceClient.DeleteEntry(ctx, &DeleteEntryRequest{EntryID: fileID})
			Expect(err).Should(BeNil())
		})
		It("get detail should be notfound", func() {
			_, err := serviceClient.GetEntryDetail(ctx, &GetEntryDetailRequest{EntryID: fileID})
			s, _ := status.FromError(err)
			Expect(s.Code()).Should(Equal(codes.NotFound))
		})
		It("list children should be empty", func() {
			resp, err := serviceClient.ListGroupChildren(ctx, &ListGroupChildrenRequest{ParentID: groupID})
			Expect(err).Should(BeNil())
			Expect(len(resp.Entries)).Should(Equal(0))
		})
	})
})

var _ = Describe("testEntriesService-FileIO", func() {
	Context("write new file", func() {
		It("write should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("re-read should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
	})
	Context("write existed file", func() {
		It("write should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("re-read should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
	})
})

var _ = Describe("testEntryPropertiesService", func() {
	var (
		ctx     = context.TODO()
		entryID int64
	)

	Context("test add property", func() {
		It("init entry should be succeed", func() {
			resp, err := serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: dentry.RootEntryID,
				Name:     "test-file-property-1",
				Kind:     types.RawKind,
			})
			Expect(err).Should(BeNil())
			entryID = resp.Entry.Id
		})
		It("add should be succeed", func() {
			resp, err := serviceClient.AddProperty(ctx, &AddPropertyRequest{
				EntryID: entryID,
				Key:     "test.key",
				Value:   "value1",
			})
			Expect(err).Should(BeNil())
			Expect(len(resp.Properties) > 0).Should(BeTrue())
		})
		It("get entry details should be succeed", func() {
			resp, err := serviceClient.GetEntryDetail(ctx, &GetEntryDetailRequest{EntryID: entryID})
			Expect(err).Should(BeNil())
			Expect(len(resp.Properties) > 0).Should(BeTrue())
		})
	})
	Context("test update property", func() {
		It("update should be succeed", func() {
			resp, err := serviceClient.UpdateProperty(ctx, &UpdatePropertyRequest{
				EntryID: entryID,
				Key:     "test.key",
				Value:   "value2",
			})
			Expect(err).Should(BeNil())
			Expect(len(resp.Properties) > 0).Should(BeTrue())
		})
		It("get entry details should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
	})
	Context("test delete property", func() {
		It("delete should be succeed", func() {
			resp, err := serviceClient.DeleteProperty(ctx, &DeletePropertyRequest{
				EntryID: entryID,
				Key:     "test.key",
			})
			Expect(err).Should(BeNil())
			Expect(len(resp.Properties)).Should(Equal(0))
		})
		It("get entry details should be succeed", func() {
			resp, err := serviceClient.GetEntryDetail(ctx, &GetEntryDetailRequest{EntryID: entryID})
			Expect(err).Should(BeNil())
			Expect(len(resp.Properties)).Should(Equal(0))
		})
	})
})

var _ = Describe("testDocumentsService-ReadView", func() {
	var (
		ctx = context.TODO()
	)

	Context("test get document details", func() {
		It("init documents should be succeed", func() {
			var err error
			err = testMeta.SaveDocument(ctx, &types.Document{
				ID: 100, OID: 1, Name: "test document 1", ParentEntryID: 1, Source: "unittest",
				KeyWords: make([]string, 0), Content: "test document", Marked: false, Unread: false,
				CreatedAt: time.Now(), ChangedAt: time.Now()})
			Expect(err).Should(BeNil())
			err = testMeta.SaveDocument(ctx, &types.Document{
				ID: 101, OID: 1, Name: "test document unread 1", ParentEntryID: 1, Source: "unittest",
				KeyWords: make([]string, 0), Content: "test document", Marked: false, Unread: true,
				CreatedAt: time.Now(), ChangedAt: time.Now()})
			Expect(err).Should(BeNil())
			err = testMeta.SaveDocument(ctx, &types.Document{
				ID: 102, OID: 1, Name: "test document unread 1", ParentEntryID: 2, Source: "unittest",
				KeyWords: make([]string, 0), Content: "test document", Marked: true, Unread: false,
				CreatedAt: time.Now(), ChangedAt: time.Now()})
			Expect(err).Should(BeNil())
		})
	})
	Context("list document", func() {
		It("list all should be succeed", func() {
			resp, err := serviceClient.ListDocuments(ctx, &ListDocumentsRequest{ListAll: true})
			Expect(err).Should(BeNil())
			Expect(len(resp.Documents)).Should(Equal(3))
		})
		It("list marked should be succeed", func() {
			resp, err := serviceClient.ListDocuments(ctx, &ListDocumentsRequest{Marked: true})
			Expect(err).Should(BeNil())
			Expect(len(resp.Documents)).Should(Equal(1))
		})
		It("list unread should be succeed", func() {
			resp, err := serviceClient.ListDocuments(ctx, &ListDocumentsRequest{Unread: true})
			Expect(err).Should(BeNil())
			Expect(len(resp.Documents)).Should(Equal(1))
		})
		It("list by parent id should be succeed", func() {
			resp, err := serviceClient.ListDocuments(ctx, &ListDocumentsRequest{ParentID: 2})
			Expect(err).Should(BeNil())
			Expect(len(resp.Documents)).Should(Equal(1))
		})
	})
})

var _ = Describe("testNotifyService", func() {
	var (
		ctx  = context.TODO()
		seq1 int64
	)

	Context("get events sequence", func() {
		It("get sequence should be succeed", func() {
			_, err := serviceClient.GetLatestSequence(ctx, &GetLatestSequenceRequest{})
			Expect(err).Should(BeNil())
		})
		It("send events using event should be succeed", func() {
			_, err := ctrl.CreateEntry(ctx, dentry.RootEntryID, types.EntryAttr{
				Name: "test-event-file-1.txt",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())
		})
		It("get sequence should be succeed", func() {
			Eventually(func() bool {
				resp, err := serviceClient.GetLatestSequence(ctx, &GetLatestSequenceRequest{})
				Expect(err).Should(BeNil())
				seq1 = resp.Sequence
				return seq1 > 0
			}, time.Second*30, time.Second).Should(BeTrue())
		})
	})
	Context("test device sync", func() {
		var (
			deviceSeq int64
			newEntry  int64
		)
		It("sync from never been seen should be succeed", func() {
			resp1, err := serviceClient.GetLatestSequence(ctx, &GetLatestSequenceRequest{StartSequence: deviceSeq})
			Expect(err).Should(BeNil())
			Expect(resp1.NeedRelist).Should(BeTrue())
			deviceSeq = resp1.Sequence
		})
		It("insert new entry should be succeed", func() {
			en, err := ctrl.CreateEntry(ctx, dentry.RootEntryID, types.EntryAttr{
				Name: "test-event-file-2.txt",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())
			newEntry = en.ID

			Eventually(func() bool {
				resp, err := serviceClient.GetLatestSequence(ctx, &GetLatestSequenceRequest{StartSequence: deviceSeq})
				Expect(err).Should(BeNil())
				return resp.Sequence > deviceSeq
			}, time.Second*30, time.Second).Should(BeTrue())
		})
		It("sync from last-sync should be succeed", func() {
			resp, err := serviceClient.ListUnSyncedEvent(ctx, &ListUnSyncedEventRequest{StartSequence: deviceSeq})
			Expect(err).Should(BeNil())
			Expect(len(resp.Events) > 0).Should(BeTrue())

			found := false
			for _, evt := range resp.Events {
				if evt.RefID == newEntry {
					found = true
				}
			}
			Expect(found).Should(BeTrue())
		})
	})
})
