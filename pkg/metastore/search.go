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

package metastore

import (
	"context"
	"runtime/trace"

	"github.com/basenana/nanafs/pkg/metastore/search"
	"github.com/basenana/nanafs/pkg/types"
)

func (s *sqlMetaStore) IndexDocument(ctx context.Context, namespace string, doc *types.IndexDocument, tokenizer func(string) []string) error {
	defer trace.StartRegion(ctx, "metastore.sql.Index").End()
	return search.IndexDocument(ctx, s.DB, namespace, doc, tokenizer)
}

func (s *sqlMetaStore) QueryDocuments(ctx context.Context, namespace, query string) ([]*types.IndexDocument, error) {
	defer trace.StartRegion(ctx, "metastore.sql.QueryLanguage").End()
	s.logger.Infow("query language", "namespace", namespace, "query", query)
	return search.QueryLanguage(ctx, s.DB, namespace, query)
}

func (s *sqlMetaStore) DeleteDocument(ctx context.Context, namespace string, id int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.Delete").End()
	return search.DeleteDocument(ctx, s.DB, namespace, id)
}

func (s *sqlMetaStore) UpdateDocumentURI(ctx context.Context, namespace string, id int64, uri string) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateDocumentURI").End()
	s.logger.Infow("update document uri", "namespace", namespace, "id", id, "uri", uri)
	return search.UpdateDocumentURI(ctx, s.DB, namespace, id, uri)
}
