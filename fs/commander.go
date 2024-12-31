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

package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
)

type Commander interface {
	FSRoot(ctx context.Context) (*types.Entry, error)
	InitNamespace(ctx context.Context, namespace string) error
	CreateEntry(ctx context.Context, namespace string, parentId int64, attr types.EntryAttr) (*types.Entry, error)
	UpdateEntry(ctx context.Context, namespace string, entry *types.Entry) error
	DestroyEntry(ctx context.Context, namespace string, parentId, entryId int64, attr types.DestroyObjectAttr) error
	MirrorEntry(ctx context.Context, namespace string, srcEntryId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error)
	ChangeEntryParent(ctx context.Context, namespace string, targetId, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error
	SetEntryEncodedProperty(ctx context.Context, namespace string, id int64, fKey string, fVal []byte) error
	SetEntryProperty(ctx context.Context, namespace string, id int64, fKey, fVal string) error
	RemoveEntryProperty(ctx context.Context, namespace string, id int64, fKey string) error
}
