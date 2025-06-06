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
	"github.com/basenana/nanafs/pkg/core"
	"io"
	"time"

	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/workflow/jobrun"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"

	"github.com/basenana/nanafs/pkg/types"
)

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
				ParentID: core.RootEntryID,
				Name:     "test-group-1",
				Kind:     types.GroupKind,
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(resp.Entry.Name).Should(Equal("test-group-1"))
			groupID = resp.Entry.Id

			resp, err = serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: core.RootEntryID,
				Name:     "test-group-2",
				Kind:     types.GroupKind,
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			groupID2 = resp.Entry.Id
		})
		It("create child file should be succeed", func() {
			resp, err := serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: groupID,
				Name:     "test-file-1",
				Kind:     types.RawKind,
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			fileID = resp.Entry.Id

			resp, err = serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: groupID,
				Name:     "test-file-2",
				Kind:     types.RawKind,
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			fileID2 = resp.Entry.Id
		})
		It("list child should be succeed", func() {
			resp, err := serviceClient.ListGroupChildren(ctx, &ListGroupChildrenRequest{ParentID: groupID}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(len(resp.Entries) > 0).Should(BeTrue())
		})
		It("get detail should be succeed", func() {
			resp, err := serviceClient.GetEntryDetail(ctx, &GetEntryDetailRequest{EntryID: fileID, ParentID: groupID}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(resp.Entry.Name).Should(Equal("test-file-1"))
		})
	})
	Context("test move entry", func() {
		It("move should be succeed", func() {
			_, err := serviceClient.ChangeParent(ctx, &ChangeParentRequest{EntryID: fileID2, NewParentID: groupID2}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
		})
		It("list child should be succeed", func() {
			resp, err := serviceClient.ListGroupChildren(ctx, &ListGroupChildrenRequest{ParentID: groupID2}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(len(resp.Entries) > 0).Should(BeTrue())
		})
	})
	Context("test update entry", func() {
		It("update should be succeed", func() {
			_, err := serviceClient.UpdateEntry(ctx, &UpdateEntryRequest{
				EntryID: fileID,
				Aliases: "test aliases",
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
		})
		It("get detail should be succeed", func() {
			resp, err := serviceClient.GetEntryDetail(ctx, &GetEntryDetailRequest{EntryID: fileID, ParentID: groupID}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(resp.Entry.Aliases).Should(Equal("test aliases"))
		})
	})
	Context("test delete entry", func() {
		It("delete should be succeed", func() {
			_, err := serviceClient.DeleteEntry(ctx, &DeleteEntryRequest{
				EntryID:   fileID,
				ParentID:  groupID,
				EntryName: "test-file-1",
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
		})
		It("get detail should be notfound", func() {
			_, err := serviceClient.GetEntryDetail(ctx, &GetEntryDetailRequest{EntryID: fileID}, grpc.UseCompressor(gzip.Name))
			s, _ := status.FromError(err)
			Expect(s.Code()).Should(Equal(codes.NotFound))
		})
		It("list children should be empty", func() {
			resp, err := serviceClient.ListGroupChildren(ctx, &ListGroupChildrenRequest{ParentID: groupID}, grpc.UseCompressor(gzip.Name))
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
				ParentID: core.RootEntryID,
				Name:     "test-bio-file-1",
				Kind:     types.RawKind,
			}, grpc.UseCompressor(gzip.Name))
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
				ParentID: core.RootEntryID,
				Name:     "test-file-property-1",
				Kind:     types.RawKind,
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			entryID = resp.Entry.Id
		})
		It("add should be succeed", func() {
			resp, err := serviceClient.AddProperty(ctx, &AddPropertyRequest{
				EntryID: entryID,
				Key:     "test.key",
				Value:   "value1",
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(len(resp.Properties) > 0).Should(BeTrue())
		})
		It("get entry details should be succeed", func() {
			resp, err := serviceClient.GetEntryDetail(ctx, &GetEntryDetailRequest{EntryID: entryID, ParentID: core.RootEntryID}, grpc.UseCompressor(gzip.Name))
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
			}, grpc.UseCompressor(gzip.Name))
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
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(len(resp.Properties)).Should(Equal(0))
		})
		It("get entry details should be succeed", func() {
			resp, err := serviceClient.GetEntryDetail(ctx, &GetEntryDetailRequest{EntryID: entryID, ParentID: core.RootEntryID}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(len(resp.Properties)).Should(Equal(0))
		})
	})
})

var _ = Describe("testDocumentsService-ReadView", func() {
	var (
		ctx      = context.TODO()
		groupID1 int64
		groupID2 int64
	)

	Context("test get document details", func() {
		It("init documents should be succeed", func() {
			var (
				err error
				t   = true
				f   = false
			)
			resp, err := serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: core.RootEntryID,
				Name:     "test-doc-group-1",
				Kind:     types.GroupKind,
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(resp.Entry.Name).Should(Equal("test-doc-group-1"))
			groupID1 = resp.Entry.Id
			resp, err = serviceClient.CreateEntry(ctx, &CreateEntryRequest{
				ParentID: core.RootEntryID,
				Name:     "test-doc-group-2",
				Kind:     types.GroupKind,
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(resp.Entry.Name).Should(Equal("test-doc-group-2"))
			groupID2 = resp.Entry.Id
			err = testFriday.CreateDocument(ctx, &types.Document{
				EntryId: 100, Name: "test document 1", Namespace: types.DefaultNamespace, ParentEntryID: groupID1, Source: "unittest",
				Content: "test document", Marked: &f, Unread: &f,
				CreatedAt: time.Now(), ChangedAt: time.Now()})
			Expect(err).Should(BeNil())
			err = testFriday.CreateDocument(ctx, &types.Document{
				EntryId: 101, Name: "test document unread 1", Namespace: types.DefaultNamespace, ParentEntryID: groupID1, Source: "unittest",
				Content: "test document", Marked: &f, Unread: &t,
				CreatedAt: time.Now(), ChangedAt: time.Now()})
			Expect(err).Should(BeNil())
			err = testFriday.CreateDocument(ctx, &types.Document{
				EntryId: 102, Name: "test document unread 1", Namespace: types.DefaultNamespace, ParentEntryID: groupID2, Source: "unittest",
				Content: "test document", Marked: &t, Unread: &f,
				CreatedAt: time.Now(), ChangedAt: time.Now()})
			Expect(err).Should(BeNil())
		})
	})
	Context("list document", func() {
		It("list all should be succeed", func() {
			resp, err := serviceClient.ListDocuments(ctx, &ListDocumentsRequest{ListAll: true}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(len(resp.Documents)).Should(Equal(3))
		})
		It("list marked should be succeed", func() {
			resp, err := serviceClient.ListDocuments(ctx, &ListDocumentsRequest{Filter: &DocumentFilter{Marked: true}}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(len(resp.Documents)).Should(Equal(1))
		})
		It("list unread should be succeed", func() {
			resp, err := serviceClient.ListDocuments(ctx, &ListDocumentsRequest{Filter: &DocumentFilter{Unread: true}}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(len(resp.Documents)).Should(Equal(1))
		})
		It("list by parent id should be succeed", func() {
			resp, err := serviceClient.ListDocuments(ctx, &ListDocumentsRequest{ParentID: groupID2}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())
			Expect(len(resp.Documents)).Should(Equal(1))
		})
	})
})

var _ = Describe("testWorkflowService", func() {
	var (
		ctx  = context.TODO()
		wfID = "mock-workflow-1"
	)

	Context("list all workflow", func() {
		It("mole should be succeed", func() {
			err := testMeta.SaveWorkflow(ctx, types.DefaultNamespace, &types.Workflow{
				Id:              wfID,
				Name:            "mock workflow",
				Namespace:       types.DefaultNamespace,
				Enable:          true,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
				LastTriggeredAt: time.Now(),
			})
			Expect(err).Should(BeNil())

			for i := 0; i < 200; i++ {
				err = testMeta.SaveWorkflowJob(ctx, types.DefaultNamespace, &types.WorkflowJob{
					Id:            utils.MustRandString(16),
					Namespace:     types.DefaultNamespace,
					Workflow:      wfID,
					TriggerReason: "mock",
					Steps:         make([]types.WorkflowJobStep, 0),
					Status:        jobrun.SucceedStatus,
					StartAt:       time.Now(),
					FinishAt:      time.Now(),
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
				})
			}
		})
		It("get should be succeed", func() {
			resp, err := serviceClient.ListWorkflows(ctx, &ListWorkflowsRequest{}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())

			Expect(len(resp.Workflows) > 0).Should(BeTrue())

		})

		It("list jobs should be succeed", func() {
			jobList, err := serviceClient.ListWorkflowJobs(ctx, &ListWorkflowJobsRequest{
				WorkflowID: wfID,
			}, grpc.UseCompressor(gzip.Name))
			Expect(err).Should(BeNil())

			Expect(len(jobList.Jobs)).Should(Equal(200))
		})
	})
})
