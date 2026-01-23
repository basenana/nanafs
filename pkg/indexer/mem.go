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

	"github.com/basenana/nanafs/pkg/types"
)

type memIndexer struct{}

func (m *memIndexer) Index(ctx context.Context, namespace string, doc *types.IndexDocument) error {
	return nil
}

func (m *memIndexer) QueryLanguage(ctx context.Context, namespace, query string) ([]*types.IndexDocument, error) {
	return []*types.IndexDocument{}, nil
}

func (m *memIndexer) Delete(ctx context.Context, namespace string, id int64) error {
	return nil
}

func (m *memIndexer) UpdateURI(ctx context.Context, namespace string, id int64, uri string) error {
	return nil
}

func (m *memIndexer) DeleteChildren(ctx context.Context, namespace string, parentID int64) error {
	return nil
}

func (m *memIndexer) UpdateChildrenURI(ctx context.Context, namespace string, parentID int64, newParentURI string) error {
	return nil
}

func NewMem() Indexer {
	return &memIndexer{}
}
