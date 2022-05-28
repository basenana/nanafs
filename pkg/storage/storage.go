package storage

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"io"
)

type Info struct {
	Key  string
	Size int64
}

type Storage interface {
	ID() string
	Get(ctx context.Context, key string, idx, offset int64) (io.ReadCloser, error)
	Put(ctx context.Context, key string, idx, offset int64, in io.Reader) error
	Delete(ctx context.Context, key string) error
	Head(ctx context.Context, key string, idx int64) (Info, error)
}

func NewStorage(storageID string, cfg config.Storage) (Storage, error) {
	switch storageID {
	case LocalStorage:
		return newLocalStorage(cfg.LocalDir), nil
	case MemoryStorage:
		return newMemoryStorage(), nil
	default:
		return nil, fmt.Errorf("unknow storage id: %s", storageID)
	}
}

type MetaStore interface {
	GetObject(ctx context.Context, id string) (*types.Object, error)
	ListObjects(ctx context.Context, filter types.Filter) ([]*types.Object, error)
	SaveObject(ctx context.Context, parent, obj *types.Object) error
	DestroyObject(ctx context.Context, src, parent, obj *types.Object) error

	ListChildren(ctx context.Context, obj *types.Object) (Iterator, error)
	MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error
	ChangeParent(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error

	SaveContent(ctx context.Context, obj *types.Object, cType types.Kind, version string, content interface{}) error
	LoadContent(ctx context.Context, obj *types.Object, cType types.Kind, version string, content interface{}) error
	DeleteContent(ctx context.Context, obj *types.Object, cType types.Kind, version string) error
}

func NewMetaStorage(metaType string, meta config.Meta) (MetaStore, error) {
	switch metaType {
	case MemoryMeta:
		return newMemoryMetaStore(), nil
	case SqliteMeta:
		return newSqliteMetaStore(meta)
	default:
		return nil, fmt.Errorf("unknow meta store type: %s", metaType)
	}
}
