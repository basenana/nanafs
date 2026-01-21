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
	"fmt"
	"os"

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/metastore/search"
	"github.com/basenana/nanafs/pkg/types"
)

var envJieBaDict = os.Getenv("STATIC_JIEBA_DICT")

type metaDB struct {
	meta metastore.Meta
}

func (m *metaDB) Index(ctx context.Context, namespace string, doc *types.IndexDocument) error {
	return m.meta.IndexDocument(ctx, namespace, doc)
}

func (m *metaDB) QueryLanguage(ctx context.Context, namespace, query string) ([]*types.IndexDocument, error) {
	return m.meta.QueryDocuments(ctx, namespace, query)
}

func (m *metaDB) Delete(ctx context.Context, namespace string, id int64) error {
	return m.meta.DeleteDocument(ctx, namespace, id)
}

func NewMetaDB(meta metastore.Meta) (Indexer, error) {
	_, err := os.Stat(envJieBaDict)
	if err != nil {
		return nil, fmt.Errorf("get jieba dict file err: %v", err)
	}
	search.SetDictPath(envJieBaDict)
	return &metaDB{meta: meta}, nil
}
