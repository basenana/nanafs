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

package controller

import (
	"context"

	friday2 "github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/types"
)

func (c *controller) ListRooms(ctx context.Context, entryId int64) ([]*types.Room, error) {
	result, err := c.dialogue.ListRooms(ctx, entryId)
	if err != nil {
		c.logger.Errorw("list rooms failed", "err", err)
		return nil, err
	}
	return result, nil
}

func (c *controller) CreateRoom(ctx context.Context, entryId int64, prompt string) (*types.Room, error) {
	result, err := c.dialogue.CreateRoom(ctx, entryId, prompt)
	if err != nil {
		c.logger.Errorw("create room failed", "err", err)
		return nil, err
	}
	return result, nil
}

func (c *controller) UpdateRoom(ctx context.Context, roomId int64, prompt string) error {
	err := c.dialogue.UpdateRoom(ctx, &types.Room{
		ID:     roomId,
		Prompt: prompt,
	})
	if err != nil {
		c.logger.Errorw("update room failed", "err", err)
	}
	return err
}

func (c *controller) GetRoom(ctx context.Context, id int64) (*types.Room, error) {
	result, err := c.dialogue.GetRoom(ctx, id)
	if err != nil {
		c.logger.Errorw("get room failed", "err", err)
		return nil, err
	}
	return result, nil
}

func (c *controller) DeleteRoom(ctx context.Context, id int64) error {
	err := c.dialogue.DeleteRoom(ctx, id)
	if err != nil {
		c.logger.Errorw("delete room failed", "err", err)
	}
	return err
}

func (c *controller) ChatInRoom(ctx context.Context, roomId int64, newMsg string, reply chan string) error {
	room, err := c.dialogue.GetRoom(ctx, roomId)
	if err != nil {
		c.logger.Errorw("get room failed", "err", err)
		return err
	}
	entry, err := c.entry.GetEntry(ctx, room.EntryId)
	if err != nil {
		c.logger.Errorw("get entry failed", "err", err)
		return err
	}
	var (
		isDir      = false
		responseCh = make(chan map[string]string)
		model      string
		respMsg    string
	)
	if entry.Kind == types.GroupKind {
		isDir = true
	}

	go func() {
		if isDir {
			err = friday2.ChatInDir(ctx, room.EntryId, []map[string]string{{"role": "user", "content": newMsg}}, responseCh)
		} else {
			err = friday2.ChatInEntry(ctx, room.EntryId, []map[string]string{{"role": "user", "content": newMsg}}, responseCh)
		}
		close(responseCh)
	}()
	if err != nil {
		return err
	}

	for line := range responseCh {
		reply <- line["content"]
		model = line["role"]
		respMsg += line["content"]
	}

	// update roomMessage
	realMsg := room.History
	realMsg = append(realMsg, map[string]string{"role": "user", "content": newMsg}, map[string]string{"role": model, "content": respMsg})
	err = c.dialogue.UpdateRoom(ctx, room)
	if err != nil {
		c.logger.Errorw("update room failed", "err", err)
		return err
	}
	err = c.dialogue.CreateMessage(ctx, roomId, newMsg, respMsg)
	if err != nil {
		c.logger.Errorw("create message failed", "err", err)
		return err
	}
	return nil
}
