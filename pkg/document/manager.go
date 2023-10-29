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
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager interface {
	ListDocuments(ctx context.Context) ([]*types.Document, error)
	CreateDocument(ctx context.Context, doc *types.Document) error
	UpdateDocument(ctx context.Context, doc *types.Document) error
	GetDocument(ctx context.Context, id string) (*types.Document, error)
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

func (m *manager) CreateDocument(ctx context.Context, doc *types.Document) error {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}
	doc.CreatedAt = time.Now()
	doc.ChangedAt = time.Now()
	if err := m.recorder.SaveDocument(ctx, doc); err != nil {
		return err
	}
	return nil
}

func (m *manager) UpdateDocument(ctx context.Context, doc *types.Document) error {
	doc.ChangedAt = time.Now()
	if err := m.recorder.SaveDocument(ctx, doc); err != nil {
		return err
	}
	return nil
}

func (m *manager) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	return m.recorder.GetDocument(ctx, id)
}

func (m *manager) DeleteDocument(ctx context.Context, id string) error {
	return m.recorder.DeleteDocument(ctx, id)
}
