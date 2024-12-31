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

package document

import (
	"context"
	"fmt"
	"runtime/trace"
	"time"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/utils"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager interface {
	ListDocuments(ctx context.Context, filter types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error)
	QueryDocuments(ctx context.Context, query string) ([]*types.Document, error)
	CreateDocument(ctx context.Context, doc *types.Document) error
	UpdateDocument(ctx context.Context, doc *types.Document) error
	GetDocument(ctx context.Context, id int64) (*types.Document, error)
	GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error)
	DeleteDocument(ctx context.Context, id int64) error
	ListDocumentGroups(ctx context.Context, parentId int64, filter types.DocFilter) ([]*types.Entry, error)
}

type manager struct {
	recorder metastore.DEntry
	entryMgr dentry.Manager
	cfg      config.Loader
	logger   *zap.SugaredLogger
	friday   friday.Friday
}

var _ Manager = &manager{}

func NewManager(recorder metastore.DEntry, entryMgr dentry.Manager, cfg config.Loader, fridayClient friday.Friday) (Manager, error) {
	docLogger := logger.NewLogger("document")
	docMgr := &manager{
		recorder: recorder,
		entryMgr: entryMgr,
		cfg:      cfg,
		logger:   docLogger,
	}
	err := registerDocExecutor(docMgr)
	if err != nil {
		return nil, err
	}
	docMgr.friday = fridayClient

	return docMgr, nil
}

func (m *manager) ListDocuments(ctx context.Context, filter types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error) {
	return m.friday.FilterDocuments(ctx, &filter, order)
}

func (m *manager) QueryDocuments(ctx context.Context, query string) ([]*types.Document, error) {
	filter := &types.DocFilter{
		Search: query,
	}
	return m.friday.FilterDocuments(ctx, filter, nil)
}

func (m *manager) UpdateDocument(ctx context.Context, doc *types.Document) error {
	return m.friday.UpdateDocument(ctx, doc)
}

func (m *manager) CreateDocument(ctx context.Context, doc *types.Document) error {
	return m.friday.CreateDocument(ctx, doc)
}

func (m *manager) GetDocument(ctx context.Context, id int64) (*types.Document, error) {
	return m.friday.GetDocument(ctx, id)
}

func (m *manager) GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error) {
	return m.friday.GetDocument(ctx, oid)
}

func (m *manager) DeleteDocument(ctx context.Context, id int64) error {
	return m.friday.DeleteDocument(ctx, id)
}

func (m *manager) ListDocumentGroups(ctx context.Context, parentId int64, filter types.DocFilter) ([]*types.Entry, error) {
	defer trace.StartRegion(ctx, "dentry.manager.ListDocumentGroups").End()
	entryFilter := types.Filter{
		ID:              parentId,
		FuzzyName:       filter.FuzzyName,
		Namespace:       types.GetNamespace(ctx).String(),
		IsGroup:         utils.ToPtr(true),
		CreatedAtStart:  filter.CreatedAtStart,
		CreatedAtEnd:    filter.CreatedAtEnd,
		ModifiedAtStart: filter.ChangedAtStart,
		ModifiedAtEnd:   filter.ChangedAtEnd,
	}
	it, err := m.recorder.FilterEntries(ctx, entryFilter)
	if err != nil {
		return nil, err
	}
	var (
		result = make([]*types.Entry, 0)
		next   *types.Entry
	)
	for it.HasNext() {
		next = it.Next()
		if next.ID == next.ParentID {
			continue
		}
		result = append(result, next)
	}
	return result, nil
}

func (m *manager) handleEntryEvent(evt *types.Event) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Hour)
	defer cancel()

	ctx = types.WithNamespace(ctx, types.NewNamespace(evt.Namespace))
	entry := evt.Data

	switch evt.Type {
	case events.ActionTypeDestroy:
		en, err := m.GetDocumentByEntryId(ctx, entry.ID)
		if err != nil {
			if err == types.ErrNotFound {
				return nil
			}
			m.logger.Errorw("[docCleanExecutor] get doc failed", "entry", entry.ID, "err", err)
			return err
		}

		return m.DeleteDocument(ctx, en.EntryId)
	case events.ActionTypeChangeParent:
		fallthrough
	case events.ActionTypeUpdate:
		en, err := m.entryMgr.GetEntry(ctx, entry.ID)
		if err != nil {
			m.logger.Errorw("[docUpdateExecutor] get entry failed", "entry", entry.ID, "err", err)
			return err
		}

		doc, err := m.GetDocumentByEntryId(ctx, en.ID)
		if err != nil {
			if err == types.ErrNotFound {
				return nil
			}
			m.logger.Errorw("[docUpdateExecutor] get doc failed", "entry", entry.ID, "err", err)
			return err
		}

		if doc.Name != en.Name || doc.ParentEntryID != en.ParentID {
			doc.Name = en.Name
			doc.ParentEntryID = en.ParentID
			err = m.UpdateDocument(ctx, doc)
			if err != nil {
				m.logger.Errorw("[docUpdateExecutor] update doc failed", "entry", entry.ID, "err", err)
				return err
			}
		}
		return nil
	case events.ActionTypeCompact:
		// do nothing
		return nil
	}

	return nil
}

func registerDocExecutor(docMgr *manager) error {
	eventMappings := []struct {
		topic   string
		action  string
		handler func(*types.Event) error
	}{
		{events.TopicNamespaceEntry, events.ActionTypeDestroy, docMgr.handleEntryEvent},
		{events.TopicNamespaceEntry, events.ActionTypeUpdate, docMgr.handleEntryEvent},
		{events.TopicNamespaceEntry, events.ActionTypeChangeParent, docMgr.handleEntryEvent},
		{events.TopicNamespaceFile, events.ActionTypeCompact, docMgr.handleEntryEvent},
	}

	for _, mapping := range eventMappings {
		if _, err := events.Subscribe(events.NamespacedTopic(mapping.topic, mapping.action), mapping.handler); err != nil {
			return fmt.Errorf("register doc event executor failed: %w", err)
		}
	}

	return nil
}
