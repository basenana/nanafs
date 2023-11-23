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

package types

import "time"

type Document struct {
	ID            string    `json:"id"`
	OID           int64     `json:"oid"`
	Name          string    `json:"name"`
	ParentEntryID int64     `json:"parent_entry_id"`
	Source        string    `json:"source"`
	KeyWords      []string  `json:"key_words,omitempty"`
	Content       string    `json:"content,omitempty"`
	Summary       string    `json:"summary,omitempty"`
	Desync        bool      `json:"desync"`
	CreatedAt     time.Time `json:"created_at"`
	ChangedAt     time.Time `json:"changed_at"`
}

func (d *Document) BleveType() string {
	return "document"
}
