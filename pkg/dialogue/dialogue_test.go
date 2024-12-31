/*
  Copyright 2024 NanaFS Authors.

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

package dialogue

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("testRoomManage", func() {
	var (
		roomId int64
		entry  *types.Entry
		err    error
	)
	Context("room", func() {
		It("create should be succeed", func() {
			entry, err = entryMgr.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name: "test_room",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())
			room, err := roomManager.CreateRoom(context.TODO(), entry.ID, "test prompt")
			Expect(err).Should(BeNil())
			roomId = room.ID
		})
		It("query should be succeed", func() {
			room, err := roomManager.GetRoom(context.TODO(), roomId)
			Expect(err).Should(BeNil())
			Expect(room.EntryId).Should(Equal(entry.ID))
		})
		It("get by entry id should be succeed", func() {
			rooms, err := roomManager.ListRooms(context.TODO(), entry.ID)
			Expect(err).Should(BeNil())
			Expect(len(rooms)).Should(Equal(1))
			Expect(rooms[0].EntryId).Should(Equal(entry.ID))
		})
		It("update should be succeed", func() {
			err := roomManager.UpdateRoom(context.TODO(), &types.Room{
				ID:    roomId,
				Title: "test title",
			})
			Expect(err).Should(BeNil())
			room, err := roomManager.GetRoom(context.TODO(), roomId)
			Expect(err).Should(BeNil())
			Expect(room.Title).Should(Equal("test title"))
			Expect(room.EntryId).Should(Equal(entry.ID))
			Expect(room.Prompt).Should(Equal("test prompt"))
		})
		It("create msg should be succeed", func() {
			_, err := roomManager.SaveMessage(context.TODO(), &types.RoomMessage{
				RoomID:    roomId,
				Sender:    "user",
				Message:   "hello",
				SendAt:    time.Now(),
				CreatedAt: time.Now(),
			})
			Expect(err).Should(BeNil())
		})
		It("list room msg should be succeed", func() {
			room, err := roomManager.GetRoom(context.TODO(), roomId)
			Expect(err).Should(BeNil())
			Expect(len(room.Messages)).Should(Equal(1))
		})
	})
	Context("delete room", func() {
		It("delete should be succeed", func() {
			err := roomManager.DeleteRoom(context.TODO(), roomId)
			Expect(err).Should(BeNil())
			room, err := roomManager.GetRoom(context.TODO(), roomId)
			Expect(err).ShouldNot(BeNil())
			Expect(room).Should(BeNil())
			msgs, err := roomManager.recorder.ListRoomMessage(context.TODO(), roomId)
			Expect(err).Should(BeNil())
			Expect(len(msgs)).Should(Equal(0))
		})
	})
})
