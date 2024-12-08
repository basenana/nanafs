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
	"strconv"
	"time"

	"github.com/basenana/nanafs/pkg/types"
)

type Document struct {
	Id        string `json:"id"`
	Namespace string `json:"namespace"`
	EntryId   string `json:"entryId"`
	Name      string `json:"name"`
	Source    string `json:"source,omitempty"`
	WebUrl    string `json:"webUrl,omitempty"`

	Content     string `json:"content"`
	Summary     string `json:"summary,omitempty"`
	HeaderImage string `json:"headerImage,omitempty"`
	SubContent  string `json:"subContent,omitempty"`

	CreatedAt int64 `json:"createdAt,omitempty"`
	UpdatedAt int64 `json:"updatedAt,omitempty"`
}

type DocumentAttr struct {
	Id        string      `json:"id"`
	Namespace string      `json:"namespace"`
	EntryId   string      `json:"entryId"`
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
}

type DocAttrRequest struct {
	Namespace string `json:"namespace"`
	EntryId   string `json:"entryId,omitempty"`
	ParentID  string `json:"parentId,omitempty"`
	UnRead    *bool  `json:"unRead,omitempty"`
	Mark      *bool  `json:"mark,omitempty"`
}

func (a *DocAttrRequest) FromType(doc *types.Document) {
	a.EntryId = strconv.Itoa(int(doc.EntryId))
	a.Namespace = doc.Namespace
	a.ParentID = strconv.Itoa(int(doc.ParentEntryID))
	a.Mark = doc.Marked
	a.UnRead = doc.Unread
}

type DocRequest struct {
	EntryId   string `json:"entryId,omitempty"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Source    string `json:"source,omitempty"`
	WebUrl    string `json:"webUrl,omitempty"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"createdAt,omitempty"`
	ChangedAt int64  `json:"changedAt,omitempty"`
}

func (r *DocRequest) FromType(doc *types.Document) {
	r.EntryId = strconv.Itoa(int(doc.EntryId))
	r.Name = doc.Name
	r.Namespace = doc.Namespace
	r.Source = doc.Source
	r.WebUrl = doc.WebUrl
	r.Content = doc.Content
	r.CreatedAt = doc.CreatedAt.Unix()
	r.ChangedAt = doc.ChangedAt.Unix()
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

	Search string `json:"search"`

	HitsPerPage int64  `json:"hitsPerPage,omitempty"`
	Page        int64  `json:"page,omitempty"`
	Limit       int64  `json:"limit,omitempty"`
	Sort        string `json:"sort,omitempty"`
	Desc        bool   `json:"desc,omitempty"`
}

type DocumentWithAttr struct {
	Document

	ParentID string `json:"parentId,omitempty"`
	UnRead   *bool  `json:"unRead,omitempty"`
	Mark     *bool  `json:"mark,omitempty"`
}

func (a *DocumentWithAttr) ToType() *types.Document {
	entryId, _ := strconv.ParseInt(a.EntryId, 10, 64)
	parentId, _ := strconv.ParseInt(a.ParentID, 10, 64)

	return &types.Document{
		EntryId:       entryId,
		Name:          a.Name,
		Namespace:     a.Namespace,
		ParentEntryID: parentId,
		Source:        a.Source,
		Content:       a.Content,
		Summary:       a.Summary,
		WebUrl:        a.WebUrl,
		HeaderImage:   a.HeaderImage,
		SubContent:    a.SubContent,
		Marked:        a.Mark,
		Unread:        a.UnRead,
		CreatedAt:     time.Unix(a.CreatedAt, 0),
		ChangedAt:     time.Unix(a.UpdatedAt, 0),
	}
}
