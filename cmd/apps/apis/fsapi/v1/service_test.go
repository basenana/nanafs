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
	"io"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("testRoomService", func() {
	var (
		ctx    = context.TODO()
		roomId int64
	)

	Context("test room", func() {
		It("create should be succeed", func() {
			resp, err := serviceClient.OpenRoom(ctx, &OpenRoomRequest{
				EntryID: 1,
				Option: &OpenRoomRequest_Option{
					Prompt: "test prompt",
				},
			})
			Expect(err).Should(BeNil())
			roomId = resp.Room.Id
		})
		It("open should be succeed", func() {
			resp, err := serviceClient.OpenRoom(ctx, &OpenRoomRequest{
				EntryID: 1,
				RoomID:  roomId,
			})
			Expect(err).Should(BeNil())
			Expect(resp.Room.Prompt).Should(Equal("test prompt"))
		})
		It("update should be succeed", func() {
			_, err := serviceClient.UpdateRoom(ctx, &UpdateRoomRequest{
				RoomID: roomId,
				Prompt: "update prompt",
			})
			Expect(err).Should(BeNil())
			res, err := serviceClient.OpenRoom(ctx, &OpenRoomRequest{
				EntryID: 1,
				RoomID:  roomId,
			})
			Expect(err).Should(BeNil())
			Expect(res.Room.Prompt).Should(Equal("update prompt"))
		})
		It("list should be succeed", func() {
			resp, err := serviceClient.ListRooms(ctx, &ListRoomsRequest{EntryID: 1})
			Expect(err).Should(BeNil())
			Expect(len(resp.Rooms)).Should(Equal(1))
		})

		It("chat should be succeed", func() {
			patch1 := ApplyFunc(friday.ChatWithEntry, func(ctx context.Context, dirId int64, isGroup bool, history []map[string]string, response chan map[string]string, historyCh chan []map[string]string) error {
				response <- map[string]string{"content": "hello, I am response\n"}
				response <- map[string]string{"content": "abc"}
				close(response)
				return nil
			})
			defer patch1.Reset()

			var (
				content   string
				requestId int64
			)
			chater, err := serviceClient.ChatInRoom(ctx, &ChatRequest{
				RoomID:     roomId,
				NewRequest: "hello",
				SendAt:     timestamppb.New(time.Now()),
			})
			Expect(err).Should(BeNil())

			for {
				resp, err := chater.Recv()
				if err != nil && err == io.EOF {
					break
				}
				Expect(err).Should(BeNil())
				if resp.RequestID != 0 {
					requestId = resp.RequestID
				}
				content += resp.ResponseMessage
			}

			Expect(content).Should(Equal("🤔hello, I am response\nabc"))

			// message should be saved
			res, err := serviceClient.OpenRoom(ctx, &OpenRoomRequest{
				EntryID: 1,
				RoomID:  roomId,
			})
			Expect(err).Should(BeNil())
			Expect(len(res.Room.Messages)).Should(Equal(2))
			Expect(res.Room.Messages[0].Message).Should(Equal("hello"))
			Expect(res.Room.Messages[0].Id).Should(Equal(requestId))
			Expect(res.Room.Messages[1].Message).Should(Equal("hello, I am response\nabc"))
		})

		It("delete should be succeed", func() {
			_, err := serviceClient.DeleteRoom(ctx, &DeleteRoomRequest{RoomID: roomId})
			Expect(err).Should(BeNil())
			res, err := serviceClient.OpenRoom(ctx, &OpenRoomRequest{
				EntryID: 1,
				RoomID:  roomId,
			})
			Expect(err).ShouldNot(BeNil())
			Expect(res).Should(BeNil())
		})
	})
})

var _ = Describe("testInboxService", func() {
	var (
		ctx        = context.TODO()
		inboxGroup int64
	)

	Context("test quick inbox", func() {
		var newEntry int64
		It("find inbox should be succeed", func() {
			en, err := ctrl.FindEntry(ctx, dentry.RootEntryID, ".inbox")
			Expect(err).Should(BeNil())
			inboxGroup = en.ID
		})
		It("inbox should be succeed", func() {
			resp, err := serviceClient.QuickInbox(ctx, &QuickInboxRequest{
				SourceType:  QuickInboxRequest_UrlSource,
				FileType:    QuickInboxRequest_WebArchiveFile,
				Filename:    "test",
				Url:         "https://blog.ihypo.net",
				ClutterFree: true,
			})
			Expect(err).Should(BeNil())

			newEntry = resp.EntryID
			Expect(newEntry > 0).Should(BeTrue())
		})
		It("file in inbox should be found", func() {
			en, err := ctrl.GetEntry(ctx, newEntry)
			Expect(err).Should(BeNil())
			Expect(en.ParentID).Should(Equal(inboxGroup))
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
	var (
		ctx    = context.TODO()
		data   = []byte("hello world!\n")
		fileID int64
	)

	Context("write new file", func() {
		It("create new file should be succeed", func() {
			resp, err := serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: dentry.RootEntryID,
				Name:     "test-bio-file-1",
				Kind:     types.RawKind,
			})
			Expect(err).Should(BeNil())
			fileID = resp.Entry.Id
		})
		It("write should be succeed", func() {
			var off int64 = 0
			writer, err := serviceClient.WriteFile(ctx)
			Expect(err).Should(BeNil())
			for i := 0; i < 3; i++ {
				err = writer.Send(&WriteFileRequest{
					EntryID: fileID,
					Off:     off,
					Len:     int64(len(data)),
					Data:    data,
				})
				Expect(err).Should(BeNil())
				off += int64(len(data))
			}
			resp, err := writer.CloseAndRecv()
			Expect(err).Should(BeNil())
			Expect(resp.Len).Should(Equal(int64(len(data) * 3)))
		})
		It("re-read should be succeed", func() {
			var (
				off     int64
				buf     = make([]byte, 5)
				content []byte
			)
			reader, err := serviceClient.ReadFile(ctx, &ReadFileRequest{
				EntryID: fileID,
				Off:     off,
				Len:     int64(len(buf)),
			})
			Expect(err).Should(BeNil())

			for {
				resp, err := reader.Recv()
				if err != nil && err == io.EOF {
					break
				}
				Expect(err).Should(BeNil())
				content = append(content, resp.Data[:resp.Len]...)
			}

			Expect(string(data) + string(data) + string(data)).Should(Equal(string(content)))
		})
	})
	Context("write existed file", func() {
		newData := []byte("hello test1!\n")
		It("write should be succeed", func() {
			writer, err := serviceClient.WriteFile(ctx)
			Expect(err).Should(BeNil())
			err = writer.Send(&WriteFileRequest{
				EntryID: fileID,
				Off:     int64(len(data)),
				Len:     int64(len(newData)),
				Data:    newData,
			})
			Expect(err).Should(BeNil())
			resp, err := writer.CloseAndRecv()
			Expect(err).Should(BeNil())
			Expect(resp.Len).Should(Equal(int64(len(newData))))
		})
		It("re-read should be succeed", func() {
			var (
				off     int64
				buf     = make([]byte, len(data))
				content []byte
			)
			reader, err := serviceClient.ReadFile(ctx, &ReadFileRequest{
				EntryID: fileID,
				Off:     off,
				Len:     int64(len(buf)),
			})
			Expect(err).Should(BeNil())

			for {
				resp, err := reader.Recv()
				if err != nil && err == io.EOF {
					break
				}
				Expect(err).Should(BeNil())
				content = append(content, resp.Data[:resp.Len]...)
			}

			Expect(string(data) + string(newData) + string(data)).Should(Equal(string(content)))
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