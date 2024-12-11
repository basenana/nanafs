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

package buildin

import (
	"context"

	"github.com/basenana/nanafs/pkg/types"
)

type Services struct {
	DocumentManager
	ExtendFieldManager
}

type ExtendFieldManager interface {
	RemoveEntry(ctx context.Context, parentId, entryId int64) error
	GetEntry(ctx context.Context, id int64) (*types.Metadata, error)

	ListEntryProperty(ctx context.Context, id int64) (types.Properties, error)
	GetEntryProperty(ctx context.Context, id int64, fKey string) (*string, bool, error)
	SetEntryProperty(ctx context.Context, id int64, fKey, fVal string, encoded bool) error
	RemoveEntryProperty(ctx context.Context, id int64, fKey string) error
}

type DocumentManager interface {
	ListDocuments(ctx context.Context, filter types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error)
	QueryDocuments(ctx context.Context, query string) ([]*types.Document, error)
	CreateDocument(ctx context.Context, doc *types.Document) error
	UpdateDocument(ctx context.Context, doc *types.Document) error
	GetDocument(ctx context.Context, id int64) (*types.Document, error)
	GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error)
	DeleteDocument(ctx context.Context, id int64) error
}
