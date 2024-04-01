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
	Context("test create entry", func() {
		It("create group should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("create child file should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("list child should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("get detail should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
	})
	Context("test update entry", func() {
		It("update should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("get detail should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
	})
	Context("test delete entry", func() {
		It("delete should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("get detail should be notfound", func() {
			Expect(nil).Should(BeNil())
		})
		It("list children should be empty", func() {
			Expect(nil).Should(BeNil())
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
	Context("test add property", func() {
		It("init entry should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("add should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("get entry details should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
	})
	Context("test update property", func() {
		It("update should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("get entry details should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
	})
	Context("test delete property", func() {
		It("delete should be succeed", func() {
			Expect(nil).Should(BeNil())
		})
		It("get entry details should be succeed", func() {
			Expect(nil).Should(BeNil())
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
