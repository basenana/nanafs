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

package db

import (
	"encoding/json"
	"github.com/basenana/nanafs/pkg/types"
	"time"
)

type SystemInfo struct {
	FsID     string `gorm:"column:fs_id;primaryKey"`
	ChunkSeg int64  `gorm:"column:chunk_seg"`
}

func (i SystemInfo) TableName() string {
	return "system_info"
}

type Object struct {
	ID         int64  `gorm:"column:id;primaryKey"`
	Name       string `gorm:"column:name;index:obj_name"`
	Aliases    string `gorm:"column:aliases"`
	ParentID   int64  `gorm:"column:parent_id;index:parent_id"`
	RefID      int64  `gorm:"column:ref_id;index:ref_id"`
	RefCount   int    `gorm:"column:ref_count"`
	Kind       string `gorm:"column:kind"`
	Hash       string `gorm:"column:hash"`
	Size       int64  `gorm:"column:size"`
	Dev        int64  `gorm:"column:dev"`
	Owner      int64  `gorm:"column:owner"`
	GroupOwner int64  `gorm:"column:group_owner"`
	Permission int64  `gorm:"column:permission"`
	Storage    string `gorm:"column:storage"`
	Namespace  string `gorm:"column:namespace;index:obj_ns"`
	CreatedAt  int64  `gorm:"column:created_at"`
	ChangedAt  int64  `gorm:"column:changed_at"`
	ModifiedAt int64  `gorm:"column:modified_at"`
	AccessAt   int64  `gorm:"column:access_at"`
}

func (o *Object) TableName() string {
	return "object"
}

func (o *Object) Update(obj *types.Object) {
	o.ID = obj.ID
	o.Name = obj.Name
	o.Aliases = obj.Aliases
	o.ParentID = obj.ParentID
	o.RefID = obj.RefID
	o.RefCount = obj.RefCount
	o.Kind = string(obj.Kind)
	o.Hash = obj.Hash
	o.Size = obj.Size
	o.Dev = obj.Dev
	o.Storage = obj.Storage
	o.Namespace = obj.Namespace
	o.CreatedAt = obj.CreatedAt.UnixNano()
	o.ChangedAt = obj.ChangedAt.UnixNano()
	o.ModifiedAt = obj.ModifiedAt.UnixNano()
	o.AccessAt = obj.AccessAt.UnixNano()
	o.Owner = obj.Access.UID
	o.GroupOwner = obj.Access.GID
	o.Permission = updateObjectPermission(obj.Access)
}

func (o *Object) Object() *types.Object {
	result := &types.Object{
		Metadata: types.Metadata{
			ID:         o.ID,
			Name:       o.Name,
			Aliases:    o.Aliases,
			ParentID:   o.ParentID,
			RefID:      o.RefID,
			RefCount:   o.RefCount,
			Kind:       types.Kind(o.Kind),
			Hash:       o.Hash,
			Size:       o.Size,
			Dev:        o.Dev,
			Storage:    o.Storage,
			Namespace:  o.Namespace,
			CreatedAt:  time.Unix(0, o.CreatedAt),
			ChangedAt:  time.Unix(0, o.ChangedAt),
			ModifiedAt: time.Unix(0, o.ModifiedAt),
			AccessAt:   time.Unix(0, o.AccessAt),
			Access:     buildObjectAccess(o.Permission, o.Owner, o.GroupOwner),
		},
	}
	return result
}

type Label struct {
	ID        int64  `gorm:"column:id;autoIncrement"`
	RefID     int64  `gorm:"column:ref_id;index:label_refid"`
	RefType   string `gorm:"column:ref_type;index:label_reftype"`
	Key       string `gorm:"column:key"`
	Value     string `gorm:"column:value"`
	SearchKey string `gorm:"column:search_key;index:label_search_key"`
}

func (o Label) TableName() string {
	return "label"
}

type ObjectProperty struct {
	ID    int64  `gorm:"column:id;autoIncrement"`
	OID   int64  `gorm:"column:oid;index:prop_oid"`
	Name  string `gorm:"column:key;index:prop_name"`
	Value string `gorm:"column:value"`
}

func (o ObjectProperty) TableName() string {
	return "object_property"
}

type ObjectExtend struct {
	ID          int64  `gorm:"column:id;primaryKey"`
	Symlink     string `gorm:"column:symlink"`
	GroupFilter []byte `gorm:"column:group_filter"`
	PlugScope   []byte `gorm:"column:plug_scope"`
}

func (o *ObjectExtend) TableName() string {
	return "object_extend"
}

func (o *ObjectExtend) Update(obj *types.Object) {
	if obj.ExtendData != nil {
		o.Symlink = obj.ExtendData.Symlink
		if obj.ExtendData.GroupFilter != nil {
			o.GroupFilter, _ = json.Marshal(obj.ExtendData.GroupFilter)
		}
		if obj.ExtendData.GroupFilter != nil {
			o.PlugScope, _ = json.Marshal(obj.ExtendData.PlugScope)
		}
	}
}

func (o *ObjectExtend) ToExtData() types.ExtendData {
	ext := types.ExtendData{
		Properties:  types.Properties{Fields: map[string]string{}},
		Symlink:     o.Symlink,
		GroupFilter: nil,
		PlugScope:   nil,
	}
	if o.GroupFilter != nil {
		_ = json.Unmarshal(o.GroupFilter, &ext.GroupFilter)
	}
	if o.PlugScope != nil {
		_ = json.Unmarshal(o.PlugScope, &ext.PlugScope)
	}
	return ext
}

type ObjectChunk struct {
	ID       int64 `gorm:"column:id;primaryKey"`
	OID      int64 `gorm:"column:oid;index:ck_oid"`
	ChunkID  int64 `gorm:"column:chunk_id;index:ck_id"`
	Off      int64 `gorm:"column:off"`
	Len      int64 `gorm:"column:len"`
	State    int16 `gorm:"column:state"`
	AppendAt int64 `gorm:"column:append_at;index:ck_append_at"`
}

func (o ObjectChunk) TableName() string {
	return "object_chunk"
}

type ScheduledTask struct {
	ID             int64     `gorm:"column:id;autoIncrement"`
	TaskID         string    `gorm:"column:task_id;index:st_task_id"`
	RefType        string    `gorm:"column:ref_type;index:sche_task_reftype"`
	RefID          int64     `gorm:"column:ref_id;index:sche_task_refid"`
	Status         string    `gorm:"column:status;index:st_task_status"`
	Result         string    `gorm:"column:result"`
	CreatedTime    time.Time `gorm:"column:created_time"`
	ExecutionTime  time.Time `gorm:"column:execution_time"`
	ExpirationTime time.Time `gorm:"column:expiration_time"`
	Event          string    `gorm:"column:event"`
}

func (d ScheduledTask) TableName() string {
	return "scheduled_task"
}

type PluginData struct {
	ID         int64            `gorm:"column:id;autoIncrement"`
	PluginName string           `gorm:"column:plugin_name;index:plugin_name"`
	Version    string           `gorm:"column:version"`
	Type       types.PluginType `gorm:"column:type"`
	GroupId    string           `gorm:"column:group_id;index:group_id"`
	RecordId   string           `gorm:"column:record_id;index:record_id"`
	Content    string           `gorm:"column:content"`
}

func (d PluginData) TableName() string {
	return "plugin_data"
}

type ObjectWorkflow struct {
	ID        string    `db:"id"`
	Synced    bool      `db:"synced"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (o ObjectWorkflow) TableName() string {
	return "object_workflow"
}
