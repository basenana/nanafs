package db

import (
	"encoding/json"
	"github.com/basenana/nanafs/pkg/types"
	"time"
)

type Object struct {
	ID         string    `db:"id"`
	Name       string    `db:"name"`
	Aliases    string    `db:"aliases"`
	ParentID   string    `db:"parent_id"`
	RefID      string    `db:"ref_id"`
	Kind       string    `db:"kind"`
	Hash       string    `db:"hash"`
	Size       int64     `db:"size"`
	Inode      uint64    `db:"inode"`
	Namespace  string    `db:"namespace"`
	CreatedAt  time.Time `db:"created_at"`
	ChangedAt  time.Time `db:"changed_at"`
	ModifiedAt time.Time `db:"modified_at"`
	AccessAt   time.Time `db:"access_at"`
	Data       []byte    `db:"data"`
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
	o.Kind = string(obj.Kind)
	o.Hash = obj.Hash
	o.Size = obj.Size
	o.Inode = obj.Inode
	o.Namespace = obj.Namespace
	o.CreatedAt = obj.CreatedAt
	o.ChangedAt = obj.ChangedAt
	o.ModifiedAt = obj.ModifiedAt
	o.AccessAt = obj.AccessAt

	o.Data, _ = json.Marshal(obj)
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
