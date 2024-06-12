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

package controller

import (
	"context"
	"runtime/trace"

	"github.com/basenana/nanafs/pkg/types"
)

func (c *controller) QueryDocuments(ctx context.Context, query string) ([]*types.Document, error) {
	return c.document.QueryDocuments(ctx, query)
}

func (c *controller) ListDocuments(ctx context.Context, filter types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error) {
	result, err := c.document.ListDocuments(ctx, filter, order)
	if err != nil {
		c.logger.Errorw("list documents failed", "err", err)
		return nil, err
	}
	return result, nil
}

func (c *controller) ListDocumentGroups(ctx context.Context, parentId int64, filter types.DocFilter) ([]*types.Metadata, error) {
	defer trace.StartRegion(ctx, "controller.ListDocumentGroups").End()
	result, err := c.document.ListDocumentGroups(ctx, parentId, filter)
	if err != nil {
		c.logger.Errorw("list document parents failed", "parent", parentId, "err", err)
		return nil, err
	}
	return result, err
}

func (c *controller) GetDocument(ctx context.Context, id int64) (*types.Document, error) {
	return c.document.GetDocument(ctx, id)
}

func (c *controller) GetDocumentsByEntryId(ctx context.Context, entryId int64) (*types.Document, error) {
	return c.document.GetDocumentByEntryId(ctx, entryId)
}

func (c *controller) UpdateDocument(ctx context.Context, doc *types.Document) error {
	return c.document.SaveDocument(ctx, doc)
}
