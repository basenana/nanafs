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

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager interface {
	ListDocuments(ctx context.Context) ([]*types.Document, error)
	SaveDocument(ctx context.Context, doc *types.Document) error
	GetDocument(ctx context.Context, id string) (*types.Document, error)
	FindDocument(ctx context.Context, uri string) (*types.Document, error)
	DeleteDocument(ctx context.Context, id string) error
}

type manager struct {
	logger   *zap.SugaredLogger
	recorder metastore.DocumentRecorder
}

var _ Manager = &manager{}

func NewManager(recorder metastore.DocumentRecorder) (Manager, error) {
	docLogger := logger.NewLogger("document")
	return &manager{
		logger:   docLogger,
		recorder: recorder,
	}, nil
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
		crtDoc, err := m.recorder.FindDocument(ctx, doc.Uri)
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
	if (doc.Name != "" && crtDoc.Name != doc.Name) || (doc.Uri != "" && crtDoc.Uri != doc.Uri) {
		return errors.New("can't update name or uri of doc")
	}
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

func (m *manager) FindDocument(ctx context.Context, uri string) (*types.Document, error) {
	return m.recorder.FindDocument(ctx, uri)
}

func (m *manager) DeleteDocument(ctx context.Context, id string) error {
	return m.recorder.DeleteDocument(ctx, id)
}
