package db

import (
	"encoding/json"
	"github.com/basenana/nanafs/pkg/types"
	"strings"
	"time"
)

type Object struct {
	ID           string    `db:"id"`
	Name         string    `db:"name"`
	Aliases      string    `db:"aliases"`
	ParentID     string    `db:"parent_id"`
	RefID        string    `db:"ref_id"`
	RefCount     int       `db:"ref_count"`
	Kind         string    `db:"kind"`
	Hash         string    `db:"hash"`
	Size         int64     `db:"size"`
	Inode        uint64    `db:"inode"`
	Dev          int64     `db:"dev"`
	Namespace    string    `db:"namespace"`
	CreatedAt    time.Time `db:"created_at"`
	ChangedAt    time.Time `db:"changed_at"`
	ModifiedAt   time.Time `db:"modified_at"`
	AccessAt     time.Time `db:"access_at"`
	ExtendData   []byte    `db:"extend_data"`
	CustomColumn []byte    `db:"custom_column"`
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
	ID          string `db:"id"`
	Uid         int64  `db:"uid"`
	Gid         int64  `db:"gid"`
	Permissions string `db:"permissions"`
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
	ID    string `db:"id"`
	Key   string `db:"key"`
	Value string `db:"value"`
}

func (o ObjectLabel) TableName() string {
	return "object_label"
}

type ObjectContent struct {
	ID      string `db:"id"`
	Kind    string `db:"kind"`
	Version string `db:"version"`
	Data    []byte `db:"data"`
}

func (o ObjectContent) TableName() string {
	return "object_content"
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
