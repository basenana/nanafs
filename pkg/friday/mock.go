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
	"context"
	"strings"

	"github.com/basenana/nanafs/pkg/types"
)

type mockFriday struct {
	docs []*types.Document
}

func NewMockFriday() Friday {
	return &mockFriday{
		docs: []*types.Document{},
	}
}

func (m *mockFriday) CreateDocument(ctx context.Context, doc *types.Document) error {
	m.docs = append(m.docs, doc)
	return nil
}

func (m *mockFriday) UpdateDocument(ctx context.Context, doc *types.Document) error {
	for i, d := range m.docs {
		if d.EntryId == doc.EntryId {
			m.docs[i] = doc
			return nil
		}
	}
	return nil
}

func (m *mockFriday) GetDocument(ctx context.Context, entryId int64) (*types.Document, error) {
	for _, d := range m.docs {
		if d.EntryId == entryId {
			return d, nil
		}
	}
	return nil, types.ErrNotFound
}

func (m *mockFriday) DeleteDocument(ctx context.Context, entryId int64) error {
	for i, d := range m.docs {
		if d.EntryId == entryId {
			m.docs = append(m.docs[:i], m.docs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockFriday) FilterDocuments(ctx context.Context, query *types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error) {
	var result []*types.Document
	for _, d := range m.docs {
		neverQuery := true
		if query.FuzzyName != "" {
			neverQuery = false
			if strings.Contains(d.Name, query.FuzzyName) {
				result = append(result, d)
				continue
			}
		}
		if query.ParentID != 0 {
			neverQuery = false
			if d.ParentEntryID == query.ParentID {
				result = append(result, d)
				continue
			}
		}
		if query.Source != "" {
			neverQuery = false
			if d.Source == query.Source {
				result = append(result, d)
				continue
			}
		}
		if query.Unread != nil {
			neverQuery = false
			if d.Unread != nil && *query.Unread == *d.Unread {
				result = append(result, d)
				continue
			}
		}
		if query.Marked != nil {
			neverQuery = false
			if d.Marked != nil && *query.Marked == *d.Marked {
				result = append(result, d)
				continue
			}
		}
		if neverQuery {
			result = append(result, d)
		}
	}
	return result, nil
}

var _ Friday = &mockFriday{}
