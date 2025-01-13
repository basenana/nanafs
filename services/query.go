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

package services

import (
	"context"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

type Query interface {
	NamespaceRoot(ctx context.Context, namespace string) (*Entry, error)
	GetGroupTree(ctx context.Context, namespace string) (*GroupTree, error)
	GetEntry(ctx context.Context, namespace string, id int64) (*Entry, error)
	ListEntryChildren(ctx context.Context, namespace string, entryId int64, order *types.EntryOrder, filters ...types.Filter) ([]*Entry, error)
}

type queryService struct {
	meta   metastore.Meta
	core   core.Core
	logger *zap.SugaredLogger
}

func newQuery(depends *Depends) (Query, error) {
	return &queryService{
		meta:   depends.Meta,
		core:   depends.Core,
		logger: logger.NewLogger("fsQuery"),
	}, nil
}

func (q *queryService) NamespaceRoot(ctx context.Context, namespace string) (*Entry, error) {
	en, err := q.meta.FindEntry(ctx, core.RootEntryID, namespace)
	if err != nil {
		return nil, err
	}
	return toEntry(en), nil
}

func (q *queryService) GetGroupTree(ctx context.Context, namespace string) (*GroupTree, error) {
	// TODO
	return nil, nil
}

func (q *queryService) GetEntry(ctx context.Context, namespace string, id int64) (*Entry, error) {
	en, err := q.meta.GetEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	return toEntry(en), nil
}

func (q *queryService) ListEntryChildren(ctx context.Context, namespace string, entryId int64, order *types.EntryOrder, filters ...types.Filter) ([]*Entry, error) {
	children, err := q.meta.ListEntryChildren(ctx, entryId, order, filters...)
	if err != nil {
		return nil, err
	}

	result := make([]*Entry, 0)
	for children.HasNext() {
		en := children.Next()
		result = append(result, toEntry(en))
	}
	return result, nil
}
