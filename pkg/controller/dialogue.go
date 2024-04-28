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
	"time"

	friday2 "github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
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

func (c *controller) CreateRoomMessage(ctx context.Context, roomID int64, sender, msg string, sendAt time.Time) (*types.RoomMessage, error) {
	result, err := c.dialogue.SaveMessage(ctx, &types.RoomMessage{
		RoomID:    roomID,
		Sender:    sender,
		Message:   msg,
		SendAt:    sendAt,
		CreatedAt: time.Now(),
	})
	if err != nil {
		c.logger.Errorw("save message failed", "err", err)
		return nil, err
	}
	return result, nil
}

func (c *controller) ChatInRoom(ctx context.Context, roomId int64, newMsg string, reply chan types.ReplyChannel) error {
	defer close(reply)
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
		isDir         = false
		responseCh    = make(chan map[string]string)
		model         string
		respMsg       string
		realHistory   = room.History
		errCh         = make(chan error, 1)
		responseMsgId = utils.GenerateNewID()
	)
	if entry.Kind == types.GroupKind {
		isDir = true
	}

	realHistory = append(realHistory, map[string]string{"role": "user", "content": newMsg})

	// update roomMessage
	room.History = realHistory
	err = c.dialogue.UpdateRoom(ctx, room)
	if err != nil {
		c.logger.Errorw("update room failed", "err", err)
		return err
	}
	if err != nil {
		c.logger.Errorw("create message failed", "err", err)
		return err
	}

	reply <- types.ReplyChannel{
		Line:       "🤔",
		ResponseId: responseMsgId,
		Sender:     "thinking",
		SendAt:     time.Now(),
		CreatedAt:  time.Now(),
	}

	go func() {
		defer close(errCh)
		realHistory, err = friday2.ChatWithEntry(ctx, room.EntryId, isDir, realHistory, responseCh)
		if err != nil {
			errCh <- err
		}
	}()

	for {
		select {
		case err = <-errCh:
			return err
		case line, ok := <-responseCh:

			if !ok {
				// update roomMessage
				realHistory = append(realHistory, map[string]string{"role": model, "content": respMsg})
				room.History = realHistory
				err = c.dialogue.UpdateRoom(ctx, room)
				if err != nil {
					c.logger.Errorw("update room failed", "err", err)
					return err
				}
			}

			if model == "" {
				model = line["role"]
			}
			respMsg += line["content"]

			// save model message
			response, err := c.dialogue.SaveMessage(ctx, &types.RoomMessage{
				ID:      responseMsgId,
				RoomID:  roomId,
				Sender:  model,
				Message: respMsg,
				SendAt:  time.Now(),
			})
			if err != nil {
				c.logger.Errorw("save message failed", "err", err)
				return err
			}
			reply <- types.ReplyChannel{
				Line:       line["content"],
				ResponseId: responseMsgId,
				Sender:     model,
				SendAt:     response.SendAt,
				CreatedAt:  response.CreatedAt,
			}
		}
	}
}
