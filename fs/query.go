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
	"github.com/basenana/nanafs/pkg/types"
)

type Query interface {
	NamespaceRoot(ctx context.Context, namespace string) (*Entry, error)
	GetGroupTree(ctx context.Context, namespace string) (*GroupEntry, error)
	FindEntry(ctx context.Context, namespace string, parentId int64, name string) (*Entry, error)
	GetEntry(ctx context.Context, namespace string, id int64) (*Entry, error)
	ListEntryChildren(ctx context.Context, namespace string, entryId int64, order *types.EntryOrder, filters ...types.Filter) ([]*Entry, error)

	BGetEntryByURI(ctx context.Context, namespace string, uri string) (*BasicEntry, error)
	BFindEntry(ctx context.Context, namespace string, parentId int64, name string) (*BasicEntry, error)
	BGetEntry(ctx context.Context, namespace string, id int64) (*BasicEntry, error)
	BListEntryChildren(ctx context.Context, namespace string, entryId int64, order *types.EntryOrder, filters ...types.Filter) ([]*BasicEntry, error)
}
