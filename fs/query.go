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

package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

type Query interface {
	NamespaceRoot(ctx context.Context, namespace string) (*Entry, error)
	GetGroupTree(ctx context.Context, namespace string) (*GroupTree, error)
	FindEntry(ctx context.Context, namespace string, parentId int64, name string) (*Entry, error)
	GetEntry(ctx context.Context, namespace string, id int64) (*Entry, error)
	ListEntryChildren(ctx context.Context, namespace string, entryId int64, order *types.EntryOrder, filters ...types.Filter) ([]*Entry, error)

	GetEntryInfoByURI(ctx context.Context, namespace string, uri string) (*EntryInfo, error)
	FindEntryInfo(ctx context.Context, namespace string, parentId int64, name string) (*EntryInfo, error)
	GetEntryInfo(ctx context.Context, namespace string, id int64) (*EntryInfo, error)
	GetEntryChildrenReader(ctx context.Context, namespace string, entryId int64) ([]*EntryInfo, error)
}

type queryService struct {
	meta   metastore.Meta
	entry  dentry.Manager
	logger *zap.SugaredLogger
}

func newQuery(depends Depends) (Query, error) {
	return &queryService{
		meta:   depends.Meta,
		entry:  depends.Entry,
		logger: logger.NewLogger("fsQuery"),
	}, nil
}

func (q *queryService) NamespaceRoot(ctx context.Context, namespace string) (*Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (q *queryService) GetGroupTree(ctx context.Context, namespace string) (*GroupTree, error) {
	//TODO implement me
	panic("implement me")
}

func (q *queryService) FindEntry(ctx context.Context, namespace string, parentId int64, name string) (*Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (q *queryService) GetEntry(ctx context.Context, namespace string, id int64) (*Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (q *queryService) ListEntryChildren(ctx context.Context, namespace string, entryId int64, order *types.EntryOrder, filters ...types.Filter) ([]*Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (q *queryService) GetEntryInfoByURI(ctx context.Context, namespace string, uri string) (*EntryInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (q *queryService) FindEntryInfo(ctx context.Context, namespace string, parentId int64, name string) (*EntryInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (q *queryService) GetEntryInfo(ctx context.Context, namespace string, id int64) (*EntryInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (q *queryService) GetEntryChildrenReader(ctx context.Context, namespace string, entryId int64) ([]*EntryInfo, error) {
	//TODO implement me
	panic("implement me")
}
