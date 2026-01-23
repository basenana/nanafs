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

package indexer

import (
	"context"
	"path"

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

type metaDB struct {
	meta   metastore.Meta
	t      Tokenizer
	logger *zap.SugaredLogger
}

func (m *metaDB) Index(ctx context.Context, namespace string, doc *types.IndexDocument) error {
	return m.meta.IndexDocument(ctx, namespace, doc, m.t.Tokenize)
}

func (m *metaDB) QueryLanguage(ctx context.Context, namespace, query string) ([]*types.IndexDocument, error) {
	return m.meta.QueryDocuments(ctx, namespace, query)
}

func (m *metaDB) Delete(ctx context.Context, namespace string, id int64) error {
	return m.meta.DeleteDocument(ctx, namespace, id)
}

func (m *metaDB) UpdateURI(ctx context.Context, namespace string, id int64, uri string) error {
	return m.meta.UpdateDocumentURI(ctx, namespace, id, uri)
}

func (m *metaDB) DeleteChildren(ctx context.Context, namespace string, parentID int64) error {
	children, err := m.meta.ListChildren(ctx, namespace, parentID)
	if err != nil {
		return err
	}

	var resultErr error
	for _, child := range children {
		if child.IsGroup {
			if err = m.DeleteChildren(ctx, namespace, child.ChildID); err != nil {
				m.logger.Warnw("delete children error", "namespace", namespace, "child", child.ChildID, "err", err)
				resultErr = err
			}
			continue
		}

		if err = m.meta.DeleteDocument(ctx, namespace, child.ChildID); err != nil {
			m.logger.Warnw("delete document error", "namespace", namespace, "child", child.ChildID, "err", err)
			resultErr = err
		}
	}
	return resultErr
}

func (m *metaDB) UpdateChildrenURI(ctx context.Context, namespace string, parentID int64, newParentURI string) error {
	children, err := m.meta.ListChildren(ctx, namespace, parentID)
	if err != nil {
		return err
	}

	var resultErr error
	for _, child := range children {
		newChildURI := path.Join(newParentURI, child.Name)

		if child.IsGroup {
			if err = m.UpdateChildrenURI(ctx, namespace, child.ChildID, newChildURI); err != nil {
				m.logger.Warnw("update children uri error", "newChildURI", newChildURI, "error", err)
				resultErr = err
			}
			continue
		}

		if err = m.meta.UpdateDocumentURI(ctx, namespace, child.ChildID, newChildURI); err != nil {
			m.logger.Warnw("update document uri error", "newChildURI", newChildURI, "error", err)
			resultErr = err
		}
	}
	return resultErr
}

func NewMetaDB(meta metastore.Meta, tokenizer Tokenizer) (Indexer, error) {
	return &metaDB{meta: meta, t: tokenizer, logger: logger.NewLogger("metaIndexer")}, nil
}
