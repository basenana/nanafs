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
	EntryId       int64     `json:"entry_id"`
	Name          string    `json:"name"`
	Namespace     string    `json:"namespace"`
	ParentEntryID int64     `json:"parent_entry_id"`
	Source        string    `json:"source"`
	Content       string    `json:"content,omitempty"`
	Summary       string    `json:"summary,omitempty"`
	WebUrl        string    `json:"web_url,omitempty"`
	HeaderImage   string    `json:"header_image,omitempty"`
	SubContent    string    `json:"sub_content,omitempty"`
	Marked        *bool     `json:"marked,omitempty"`
	Unread        *bool     `json:"unread,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	ChangedAt     time.Time `json:"changed_at"`
}

type DocFilter struct {
	Search         string
	FuzzyName      string
	ParentID       int64
	Source         string
	Marked         *bool
	Unread         *bool
	CreatedAtStart *time.Time
	CreatedAtEnd   *time.Time
	ChangedAtStart *time.Time
	ChangedAtEnd   *time.Time
}

type DocumentOrder struct {
	Order DocOrder
	Desc  bool
}

type DocOrder int

const (
	Name DocOrder = iota
	Source
	Marked
	Unread
	CreatedAt
)

func (d DocOrder) String() string {
	names := []string{
		"name",
		"source",
		"marked",
		"unread",
		"created_at",
	}
	if d < Name || d > CreatedAt {
		return ""
	}
	return names[d]
}
