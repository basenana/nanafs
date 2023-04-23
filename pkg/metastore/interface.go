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
	"github.com/basenana/nanafs/pkg/types"
)

type Meta interface {
	ObjectStore
	ChunkStore
	PluginRecorderGetter
}

type ObjectStore interface {
	GetObject(ctx context.Context, id int64) (*types.Object, error)
	GetObjectExtendData(ctx context.Context, obj *types.Object) error
	ListObjects(ctx context.Context, filter types.Filter) ([]*types.Object, error)
	SaveObject(ctx context.Context, parent, obj *types.Object) error
	DestroyObject(ctx context.Context, src, parent, obj *types.Object) error

	ListChildren(ctx context.Context, obj *types.Object) (Iterator, error)
	MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error
	ChangeParent(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error
}

type ChunkStore interface {
	NextSegmentID(ctx context.Context) (int64, error)
	ListSegments(ctx context.Context, oid, chunkID int64) ([]types.ChunkSeg, error)
	AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Object, error)
	DeleteSegment(ctx context.Context, segID int64) error
}

type PluginRecorderGetter interface {
	PluginRecorder(plugin types.PlugScope) PluginRecorder
}

type PluginRecorder interface {
	GetRecord(ctx context.Context, rid string, record interface{}) error
	ListRecords(ctx context.Context, groupId string) ([]string, error)
	SaveRecord(ctx context.Context, groupId, rid string, record interface{}) error
	DeleteRecord(ctx context.Context, rid string) error
}
