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

package friday

import (
	"time"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

type Document struct {
	EntryId       int64     `json:"entry_id"`
	Name          string    `json:"name"`
	Namespace     string    `json:"namespace"`
	ParentEntryID *int64    `json:"parent_entry_id"`
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

func (d *Document) FromType(doc *types.Document) *Document {
	d.EntryId = doc.EntryId
	d.Name = doc.Name
	d.Namespace = doc.Namespace
	d.Source = doc.Source
	d.Content = doc.Content
	d.Summary = doc.Summary
	d.WebUrl = doc.WebUrl
	d.HeaderImage = doc.HeaderImage
	d.SubContent = doc.SubContent
	d.Marked = doc.Marked
	d.Unread = doc.Unread
	d.CreatedAt = doc.CreatedAt
	d.ChangedAt = doc.ChangedAt
	d.ParentEntryID = &doc.ParentEntryID
	return d
}

func (d *Document) ToType() *types.Document {
	doc := &types.Document{
		EntryId:     d.EntryId,
		Name:        d.Name,
		Namespace:   d.Namespace,
		Source:      d.Source,
		Content:     d.Content,
		Summary:     d.Summary,
		WebUrl:      d.WebUrl,
		HeaderImage: d.HeaderImage,
		SubContent:  d.SubContent,
		Marked:      d.Marked,
		Unread:      d.Unread,
		CreatedAt:   d.CreatedAt,
		ChangedAt:   d.ChangedAt,
	}
	if d.ParentEntryID != nil {
		doc.ParentEntryID = *d.ParentEntryID
	}
	if d.Marked == nil {
		doc.Marked = utils.ToPtr(false)
	}
	if d.Unread == nil {
		doc.Unread = utils.ToPtr(true)
	}
	return doc
}

type DocUpdateRequest struct {
	Namespace string `json:"namespace"`
	EntryId   int64  `json:"entryId,omitempty"`
	ParentID  *int64 `json:"parentId,omitempty"`
	UnRead    *bool  `json:"unRead,omitempty"`
	Mark      *bool  `json:"mark,omitempty"`
}

func (r *DocUpdateRequest) FromType(doc *types.Document) *DocUpdateRequest {
	r.Namespace = doc.Namespace
	r.EntryId = doc.EntryId
	if doc.ParentEntryID != 0 {
		r.ParentID = &doc.ParentEntryID
	}
	r.Mark = doc.Marked
	r.UnRead = doc.Unread
	return r
}

type DocQuery struct {
	IDs            []string `json:"ids"`
	Namespace      string   `json:"namespace"`
	Source         string   `json:"source,omitempty"`
	WebUrl         string   `json:"webUrl,omitempty"`
	ParentID       string   `json:"parentId,omitempty"`
	UnRead         *bool    `json:"unRead,omitempty"`
	Mark           *bool    `json:"mark,omitempty"`
	CreatedAtStart *int64   `json:"createdAtStart,omitempty"`
	CreatedAtEnd   *int64   `json:"createdAtEnd,omitempty"`
	ChangedAtStart *int64   `json:"changedAtStart,omitempty"`
	ChangedAtEnd   *int64   `json:"changedAtEnd,omitempty"`
	FuzzyName      *string  `json:"fuzzyName,omitempty"`

	Search string `json:"search,omitempty"`

	HitsPerPage int64  `json:"hitsPerPage,omitempty"`
	Page        int64  `json:"page,omitempty"`
	Limit       int64  `json:"limit,omitempty"`
	Sort        string `json:"sort,omitempty"`
	Desc        bool   `json:"desc,omitempty"`
}
