package db

import (
	"encoding/json"
	"github.com/basenana/nanafs/pkg/types"
	"strings"
	"time"
)

var dbModels = []interface{}{
	&SystemInfo{},
	&Object{},
	&ObjectAccess{},
	&ObjectLabel{},
	&PluginData{},
}

type SystemInfo struct {
	FsID  string `gorm:"column:fs_id;primaryKey"`
	Inode uint64 `gorm:"column:inode"`
}

func (i SystemInfo) TableName() string {
	return "system_info"
}

type Object struct {
	ID           int64     `gorm:"column:id;primaryKey"`
	Name         string    `gorm:"column:name"`
	Aliases      string    `gorm:"column:aliases"`
	ParentID     int64     `gorm:"column:parent_id;index:parent_id"`
	RefID        int64     `gorm:"column:ref_id;index:ref_id"`
	RefCount     int       `gorm:"column:ref_count"`
	Kind         string    `gorm:"column:kind"`
	Hash         string    `gorm:"column:hash"`
	Size         int64     `gorm:"column:size"`
	Inode        uint64    `gorm:"column:inode;unique"`
	Dev          int64     `gorm:"column:dev"`
	Namespace    string    `gorm:"column:namespace"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	ChangedAt    time.Time `gorm:"column:changed_at"`
	ModifiedAt   time.Time `gorm:"column:modified_at"`
	AccessAt     time.Time `gorm:"column:access_at"`
	ExtendData   []byte    `gorm:"column:extend_data"`
	CustomColumn []byte    `gorm:"column:custom_column"`
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
	o.Inode = obj.Inode
	o.Dev = obj.Dev
	o.Namespace = obj.Namespace
	o.CreatedAt = obj.CreatedAt
	o.ChangedAt = obj.ChangedAt
	o.ModifiedAt = obj.ModifiedAt
	o.AccessAt = obj.AccessAt

	o.ExtendData, _ = json.Marshal(obj.ExtendData)
	o.CustomColumn, _ = json.Marshal(obj.CustomColumn)
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
			Inode:      o.Inode,
			Dev:        o.Dev,
			Namespace:  o.Namespace,
			CreatedAt:  o.CreatedAt,
			ChangedAt:  o.ChangedAt,
			ModifiedAt: o.ModifiedAt,
			AccessAt:   o.AccessAt,
		},
	}
	_ = json.Unmarshal(o.ExtendData, &(result.ExtendData))
	_ = json.Unmarshal(o.CustomColumn, &(result.CustomColumn))
	return result
}

type ObjectAccess struct {
	ID          int64  `gorm:"column:id;primaryKey"`
	Uid         int64  `gorm:"column:uid"`
	Gid         int64  `gorm:"column:gid"`
	Permissions string `gorm:"column:permissions"`
}

func (a *ObjectAccess) TableName() string {
	return "object_access"
}

func (a *ObjectAccess) Update(obj *types.Object) {
	acc := obj.Access
	a.ID = obj.ID
	a.Uid = acc.UID
	a.Gid = acc.GID

	pList := make([]string, 0, len(acc.Permissions))
	for _, p := range acc.Permissions {
		pList = append(pList, string(p))
	}
	a.Permissions = strings.Join(pList, ",")
}

func (a *ObjectAccess) ToAccess() types.Access {
	result := types.Access{
		UID: a.Uid,
		GID: a.Gid,
	}
	permissions := strings.Split(a.Permissions, ",")
	for _, p := range permissions {
		result.Permissions = append(result.Permissions, types.Permission(p))
	}
	return result
}

type ObjectLabel struct {
	ID        int64  `gorm:"column:id;autoIncrement"`
	OID       int64  `gorm:"column:oid;index:oid"`
	Key       string `gorm:"column:key"`
	Value     string `gorm:"column:value"`
	SearchKey string `gorm:"column:search_key;index:search_key"`
}

func (o ObjectLabel) TableName() string {
	return "object_label"
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
