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

package types

import "time"

type Room struct {
	ID        int64               `json:"id"`
	Namespace string              `json:"namespace"`
	Title     string              `json:"title"`
	EntryId   int64               `json:"entry_id"`
	Prompt    string              `json:"prompt"`
	History   []map[string]string `json:"history"`
	CreatedAt time.Time           `json:"created_at"`
	Messages  []*RoomMessage      `json:"messages"`
}

type RoomMessage struct {
	ID        int64     `json:"id"`
	Namespace string    `json:"namespace"`
	RoomID    int64     `json:"room_id"`
	UserMsg   string    `json:"user_msg"`
	ModelMsg  string    `json:"model_msg"`
	CreatedAt time.Time `json:"created_at"`
}
