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
	"errors"
	"fmt"
	"github.com/basenana/nanafs/config"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	labelKeyGroupFeedID = "org.basenana.internal.feed/id"
)

type Manager interface {
	ListDocuments(ctx context.Context, parentId int64) ([]*types.Document, error)
	QueryDocuments(ctx context.Context, query string) ([]*types.Document, error)
	SaveDocument(ctx context.Context, doc *types.Document) error
	GetDocument(ctx context.Context, id int64) (*types.Document, error)
	GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error)
	DeleteDocument(ctx context.Context, id int64) error

	EnableGroupFeed(ctx context.Context, id int64, feedID string) error
	DisableGroupFeed(ctx context.Context, id int64) error
	GetGroupByFeedId(ctx context.Context, feedID string) (*types.Metadata, error)
	GetDocsByFeedId(ctx context.Context, feedID string, count int) ([]*types.Document, error)
}

type manager struct {
	recorder metastore.DEntry
	entryMgr dentry.Manager
	indexer  *Indexer
	logger   *zap.SugaredLogger
}

var _ Manager = &manager{}

func NewManager(recorder metastore.DEntry, entryMgr dentry.Manager, indexerCfg *config.Indexer) (Manager, error) {
	docLogger := logger.NewLogger("document")
	docMgr := &manager{
		logger:   docLogger,
		recorder: recorder,
		entryMgr: entryMgr,
	}
	err := registerDocExecutor(docMgr)
	if err != nil {
		return nil, err
	}

	if indexerCfg != nil {
		docMgr.indexer, err = NewDocumentIndexer(recorder, *indexerCfg)
		if err != nil {
			return nil, err
		}
	}

	return docMgr, nil
}

func (m *manager) ListDocuments(ctx context.Context, parentId int64) ([]*types.Document, error) {
	result, err := m.recorder.ListDocument(ctx, parentId)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) QueryDocuments(ctx context.Context, query string) ([]*types.Document, error) {
	if m.indexer == nil {
		return nil, fmt.Errorf("indexer not enable")
	}
	return m.indexer.Query(ctx, query, QueryDialectBleve)
}

func (m *manager) SaveDocument(ctx context.Context, doc *types.Document) error {
	if doc.ID == 0 {
		crtDoc, err := m.recorder.GetDocumentByEntryId(ctx, doc.OID)
		if err != nil {
			if errors.Is(err, types.ErrNotFound) {
				// create new one
				doc.ID = utils.GenerateNewID()
				doc.CreatedAt = time.Now()
				doc.ChangedAt = time.Now()
				if err = m.recorder.SaveDocument(ctx, doc); err != nil {
					m.logger.Errorw("create document failed", "document", doc.ID, "err", err)
					return err
				}
				m.publicDocActionEvent(events.ActionTypeCreate, doc)
				return nil
			}
			m.logger.Errorw("create document failed", "err", err)
			return err
		}

		// update
		crtDoc.ChangedAt = time.Now()
		crtDoc.Summary = doc.Summary
		crtDoc.KeyWords = doc.KeyWords
		crtDoc.Content = doc.Content
		crtDoc.Desync = doc.Desync
		if err = m.recorder.SaveDocument(ctx, crtDoc); err != nil {
			m.logger.Errorw("update document failed", "document", crtDoc.ID, "err", err)
			return err
		}
		m.publicDocActionEvent(events.ActionTypeUpdate, doc)
		return nil
	}
	// update
	crtDoc, err := m.recorder.GetDocument(ctx, doc.ID)
	if err != nil {
		return err
	}
	if doc.OID != 0 && crtDoc.OID != doc.OID {
		return errors.New("can't update oid of doc")
	}
	crtDoc.Name = doc.Name
	crtDoc.ParentEntryID = doc.ParentEntryID
	crtDoc.ChangedAt = time.Now()
	crtDoc.Summary = doc.Summary
	crtDoc.KeyWords = doc.KeyWords
	crtDoc.Content = doc.Content
	crtDoc.Desync = doc.Desync
	if err = m.recorder.SaveDocument(ctx, crtDoc); err != nil {
		m.logger.Errorw("update document failed", "document", doc.ID, "err", err)
		return err
	}
	m.publicDocActionEvent(events.ActionTypeUpdate, doc)
	return nil
}

func (m *manager) GetDocument(ctx context.Context, id int64) (*types.Document, error) {
	return m.recorder.GetDocument(ctx, id)
}

func (m *manager) GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error) {
	return m.recorder.GetDocumentByEntryId(ctx, oid)
}

func (m *manager) DeleteDocument(ctx context.Context, id int64) error {
	doc, err := m.GetDocument(ctx, id)
	if err != nil {
		return err
	}
	err = m.recorder.DeleteDocument(ctx, id)
	if err != nil {
		return err
	}
	m.publicDocActionEvent(events.ActionTypeDestroy, doc)
	return nil
}

func (m *manager) GetGroupByFeedId(ctx context.Context, feedID string) (*types.Metadata, error) {
	entriesIt, err := m.recorder.FilterEntries(ctx, types.Filter{
		Label: types.LabelMatch{Include: []types.Label{{
			Key:   labelKeyGroupFeedID,
			Value: feedID,
		}}},
	})
	if err != nil {
		m.logger.Errorw("get group by feed id failed when query labels", "feedid", feedID, "err", err)
		return nil, err
	}

	var entry *types.Metadata
	for entriesIt.HasNext() {
		// take the first one
		entry = entriesIt.Next()
		break
	}

	if entry == nil {
		m.logger.Errorw("get group by feed id failed when get entry", "feedid", feedID, "err", err)
		return nil, types.ErrNotFound
	}

	if !types.IsGroup(entry.Kind) {
		m.logger.Errorw("entry is not group when get group by feed id failed", "feedid", feedID, "entryid", entry.ID, "err", err)
		return nil, types.ErrNoGroup
	}
	return entry, nil
}

func (m *manager) GetDocsByFeedId(ctx context.Context, feedID string, count int) ([]*types.Document, error) {
	entriesIt, err := m.recorder.FilterEntries(ctx, types.Filter{
		Label: types.LabelMatch{Include: []types.Label{{
			Key:   labelKeyGroupFeedID,
			Value: feedID,
		}}},
	})
	if err != nil {
		m.logger.Errorw("get docs by feed id failed when query labels", "feedid", feedID, "err", err)
		return nil, err
	}

	var entry *types.Metadata
	for entriesIt.HasNext() {
		// take the first one
		entry = entriesIt.Next()
		break
	}

	if entry == nil {
		m.logger.Errorw("get docs by feed id failed when get entry", "feedid", feedID, "err", err)
		return nil, types.ErrNotFound
	}

	if !types.IsGroup(entry.Kind) {
		m.logger.Errorw("entry is not group when get docs by feed id failed", "feedid", feedID, "entryid", entry.ID, "err", err)
		return nil, types.ErrNoGroup
	}

	documents, err := m.recorder.ListDocument(ctx, entry.ID)
	if err != nil {
		return nil, err
	}

	if len(documents) < count {
		return documents, nil
	}
	return documents[:count], nil
}

func (m *manager) EnableGroupFeed(ctx context.Context, id int64, feedID string) error {
	labels, err := m.entryMgr.GetEntryLabels(ctx, id)
	if err != nil {
		m.logger.Errorw("enable group feed failed when query labels", "entry", id, "err", err)
		return err
	}

	var (
		updated, found bool
	)
	for _, l := range labels.Labels {
		if l.Key == labelKeyGroupFeedID {
			found = true
			if l.Value != feedID {
				l.Value = feedID
				updated = true
			}
			break
		}
	}

	if !found {
		labels.Labels = append(labels.Labels, types.Label{Key: labelKeyGroupFeedID, Value: feedID})
		updated = true
	}

	if updated {
		err = m.entryMgr.UpdateEntryLabels(ctx, id, labels)
		if err != nil {
			m.logger.Errorw("enable group feed failed when write back labels", "entry", id, "err", err)
			return err
		}
	}
	return nil
}

func (m *manager) DisableGroupFeed(ctx context.Context, id int64) error {
	labels, err := m.entryMgr.GetEntryLabels(ctx, id)
	if err != nil {
		m.logger.Errorw("disable group feed failed when query lables", "entry", id, "err", err)
		return err
	}

	var (
		idx   int
		total = len(labels.Labels)
	)
	for idx = 0; idx < total; idx++ {
		if labels.Labels[idx].Key == labelKeyGroupFeedID {
			break
		}
	}

	if idx == total {
		return nil
	}

	labels.Labels = append(labels.Labels[0:idx], labels.Labels[idx+1:total]...)

	err = m.entryMgr.UpdateEntryLabels(ctx, id, labels)
	if err != nil {
		m.logger.Errorw("disable group feed failed when write back labels", "entry", id, "err", err)
		return err
	}
	return nil
}

func (m *manager) handleEntryEvent(evt *types.EntryEvent) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Hour)
	defer cancel()

	entry := evt.Data

	switch evt.Type {
	case events.ActionTypeDestroy:
		doc, err := m.GetDocumentByEntryId(ctx, entry.ID)
		if err != nil {
			if err == types.ErrNotFound {
				return nil
			}
			m.logger.Errorw("[docCleanExecutor] get doc failed", "entry", entry.ID, "err", err)
			return err
		}

		err = m.DeleteDocument(ctx, doc.ID)
		if err != nil {
			m.logger.Errorw("[docCleanExecutor] delete doc failed", "entry", entry.ID, "document", doc.ID, "err", err)
			return err
		}
		return nil
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
			err = m.SaveDocument(ctx, doc)
			if err != nil {
				m.logger.Errorw("[docUpdateExecutor] update doc failed", "entry", entry.ID, "document", doc.ID, "err", err)
				return err
			}
		}
		return nil
	case events.ActionTypeCompact:
		doc, err := m.GetDocumentByEntryId(ctx, entry.ID)
		if err != nil {
			if err == types.ErrNotFound {
				return nil
			}
			m.logger.Errorw("[docDesyncExecutor] get entry failed", "entry", entry.ID, "err", err)
			return err
		}
		if doc.Desync {
			return nil
		}
		doc.Desync = true
		err = m.SaveDocument(ctx, doc)
		if err != nil {
			m.logger.Errorw("[docDesyncExecutor] update doc failed", "entry", entry.ID, "document", doc.ID, "err", err)
			return err
		}
		return nil
	}

	return nil
}

func registerDocExecutor(docMgr *manager) error {
	eventMappings := []struct {
		topic   string
		action  string
		handler func(*types.EntryEvent) error
	}{
		{events.TopicNamespaceEntry, events.ActionTypeDestroy, docMgr.handleEntryEvent},
		{events.TopicNamespaceEntry, events.ActionTypeUpdate, docMgr.handleEntryEvent},
		{events.TopicNamespaceEntry, events.ActionTypeChangeParent, docMgr.handleEntryEvent},
		{events.TopicNamespaceFile, events.ActionTypeCompact, docMgr.handleEntryEvent},
		{events.TopicNamespaceDocument, events.ActionTypeCreate, docMgr.handleDocumentEvent},
		{events.TopicNamespaceDocument, events.ActionTypeUpdate, docMgr.handleDocumentEvent},
		{events.TopicNamespaceDocument, events.ActionTypeDestroy, docMgr.handleDocumentEvent},
	}

	for _, mapping := range eventMappings {
		if _, err := events.Subscribe(events.NamespacedTopic(mapping.topic, mapping.action), mapping.handler); err != nil {
			return fmt.Errorf("register doc event executor failed: %w", err)
		}
	}

	return nil
}
