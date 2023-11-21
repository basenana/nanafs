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
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager interface {
	ListDocuments(ctx context.Context) ([]*types.Document, error)
	SaveDocument(ctx context.Context, doc *types.Document) error
	GetDocument(ctx context.Context, id string) (*types.Document, error)
	GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error)
	DeleteDocument(ctx context.Context, id string) error
}

type manager struct {
	logger   *zap.SugaredLogger
	recorder metastore.DocumentRecorder
	entryMgr dentry.Manager
}

var _ Manager = &manager{}

func NewManager(recorder metastore.DocumentRecorder, entryMgr dentry.Manager) (Manager, error) {
	docLogger := logger.NewLogger("document")
	docMgr := &manager{
		logger:   docLogger,
		recorder: recorder,
		entryMgr: entryMgr,
	}
	err := registerDocExecutor(docMgr)
	return docMgr, err
}

func (m *manager) ListDocuments(ctx context.Context) ([]*types.Document, error) {
	result, err := m.recorder.ListDocument(ctx)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) SaveDocument(ctx context.Context, doc *types.Document) error {
	if doc.ID == "" {
		crtDoc, err := m.recorder.GetDocumentByEntryId(ctx, doc.OID)
		if err != nil {
			if errors.Is(err, types.ErrNotFound) {
				// create new one
				doc.ID = uuid.New().String()
				doc.CreatedAt = time.Now()
				doc.ChangedAt = time.Now()
				return m.recorder.SaveDocument(ctx, doc)
			}
			return err
		}

		// update
		crtDoc.ChangedAt = time.Now()
		crtDoc.Summary = doc.Summary
		crtDoc.KeyWords = doc.KeyWords
		crtDoc.Content = doc.Content
		crtDoc.Desync = doc.Desync
		return m.recorder.SaveDocument(ctx, crtDoc)
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
	return m.recorder.SaveDocument(ctx, crtDoc)
}

func (m *manager) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	return m.recorder.GetDocument(ctx, id)
}

func (m *manager) GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error) {
	return m.recorder.GetDocumentByEntryId(ctx, oid)
}

func (m *manager) DeleteDocument(ctx context.Context, id string) error {
	return m.recorder.DeleteDocument(ctx, id)
}

func (m *manager) handleEvent(evt *types.EntryEvent) error {
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
	if _, err := events.Subscribe(events.EntryActionTopic(events.TopicNamespaceEntry, events.ActionTypeDestroy), docMgr.handleEvent); err != nil {
		return err
	}
	if _, err := events.Subscribe(events.EntryActionTopic(events.TopicNamespaceEntry, events.ActionTypeUpdate), docMgr.handleEvent); err != nil {
		return err
	}
	if _, err := events.Subscribe(events.EntryActionTopic(events.TopicNamespaceEntry, events.ActionTypeChangeParent), docMgr.handleEvent); err != nil {
		return err
	}
	if _, err := events.Subscribe(events.EntryActionTopic(events.TopicNamespaceFile, events.ActionTypeCompact), docMgr.handleEvent); err != nil {
		return err
	}
	return nil
}
