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
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager interface {
	ListRooms(ctx context.Context, entryId int64) ([]*types.Room, error)
	CreateRoom(ctx context.Context, entryId int64, prompt string) (*types.Room, error)
	UpdateRoom(ctx context.Context, room *types.Room) error
	GetRoom(ctx context.Context, id int64) (*types.Room, error)
	FindRoom(ctx context.Context, entryId int64) (*types.Room, error)
	DeleteRoom(ctx context.Context, id int64) error
	DeleteRoomMessages(ctx context.Context, roomId int64) error
	SaveMessage(ctx context.Context, roomMessage *types.RoomMessage) (*types.RoomMessage, error)
}

type manager struct {
	recorder metastore.EntryStore
	logger   *zap.SugaredLogger
}

var _ Manager = &manager{}

func NewManager(recorder metastore.EntryStore) (Manager, error) {
	roomLogger := logger.NewLogger("room")
	roomMgr := &manager{
		logger:   roomLogger,
		recorder: recorder,
	}

	return roomMgr, nil
}

func (m *manager) ListRooms(ctx context.Context, entryId int64) ([]*types.Room, error) {
	result, err := m.recorder.ListRooms(ctx, entryId)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) CreateRoom(ctx context.Context, entryId int64, prompt string) (*types.Room, error) {
	room := &types.Room{
		ID:      utils.GenerateNewID(),
		EntryId: entryId,
		Prompt:  prompt,
		//History:   []map[string]string{},
		Namespace: types.GetNamespace(ctx).String(),
		CreatedAt: time.Now(),
	}
	return room, m.recorder.SaveRoom(ctx, room)
}

func (m *manager) UpdateRoom(ctx context.Context, room *types.Room) error {
	crt, err := m.recorder.GetRoom(ctx, room.ID)
	if err != nil {
		return err
	}
	if room.Title != "" {
		crt.Title = room.Title
	}
	if room.Prompt != "" {
		crt.Prompt = room.Prompt
	}
	if room.History != nil {
		crt.History = room.History
	}
	return m.recorder.SaveRoom(ctx, crt)
}

func (m *manager) GetRoom(ctx context.Context, id int64) (*types.Room, error) {
	room, err := m.recorder.GetRoom(ctx, id)
	if err != nil {
		return nil, err
	}
	msgs, err := m.recorder.ListRoomMessage(ctx, id)
	if err != nil {
		return nil, err
	}
	room.Messages = msgs
	return room, nil
}

func (m *manager) FindRoom(ctx context.Context, entryId int64) (*types.Room, error) {
	room, err := m.recorder.FindRoom(ctx, entryId)
	if err != nil {
		return nil, err
	}
	msgs, err := m.recorder.ListRoomMessage(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	room.Messages = msgs
	return room, nil
}

func (m *manager) DeleteRoom(ctx context.Context, id int64) error {
	err := m.recorder.DeleteRoomMessages(ctx, id)
	if err != nil {
		return err
	}
	return m.recorder.DeleteRoom(ctx, id)
}

func (m *manager) DeleteRoomMessages(ctx context.Context, roomId int64) error {
	return m.recorder.DeleteRoomMessages(ctx, roomId)
}

func (m *manager) SaveMessage(ctx context.Context, roomMessage *types.RoomMessage) (*types.RoomMessage, error) {
	if roomMessage.ID == 0 {
		roomMessage.ID = utils.GenerateNewID()
	}
	crtMsg, err := m.recorder.GetRoomMessage(ctx, roomMessage.ID)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, err
	}

	if crtMsg == nil {
		roomMessage.CreatedAt = time.Now()
		return roomMessage, m.recorder.SaveRoomMessage(ctx, roomMessage)
	}

	if roomMessage.Message != "" {
		crtMsg.Message = roomMessage.Message
	}

	return crtMsg, m.recorder.SaveRoomMessage(ctx, crtMsg)
}
