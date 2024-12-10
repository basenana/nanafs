/*
  Copyright 2024 NanaFS Authors.

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

package friday

import (
	"context"

	"github.com/basenana/nanafs/pkg/types"
)

type Friday interface {
	CreateDocument(ctx context.Context, doc *types.Document) error
	UpdateDocument(ctx context.Context, doc *types.Document) error
	GetDocument(ctx context.Context, entryId int64) (*types.Document, error)
	DeleteDocument(ctx context.Context, entryId int64) error
	FilterDocuments(ctx context.Context, query *types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error)
}
