package storage

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"io"
)

const (
	defaultChunkSize = 1 << 22 // 4MB
)

type Info struct {
	Key  string
	Size int64
}

type Storage interface {
	ID() string
	Get(ctx context.Context, key string, idx, off, limit int64) (io.ReadCloser, error)
	Put(ctx context.Context, key string, in io.Reader, idx, off int64) error
	Delete(ctx context.Context, key string) error
	Fsync(ctx context.Context, key string) error
	Head(ctx context.Context, key string) (Info, error)
}

func NewStorage(storageID string, cfg config.Storage) (Storage, error) {
	switch storageID {
	case localStorageID:
		return newLocalStorage(cfg.LocalDir), nil
	case memoryStorageID:
		return newMemoryStorage(), nil
	default:
		return nil, fmt.Errorf("unknow storage id: %s", storageID)
	}
}

type MetaStore interface {
	GetObject(ctx context.Context, id string) (*types.Object, error)
	ListObjects(ctx context.Context, filter Filter) ([]*types.Object, error)
	SaveObject(ctx context.Context, obj *types.Object) error
	DestroyObject(ctx context.Context, obj *types.Object) error

	ListChildren(ctx context.Context, id string) (Iterator, error)
	ChangeParent(ctx context.Context, old *types.Object, parent *types.Object) error

	SaveContent(ctx context.Context, obj *types.Object, cType, version string, content interface{}) error
	LoadContent(ctx context.Context, obj *types.Object, cType, version string, content interface{}) error
	DeleteContent(ctx context.Context, obj *types.Object, cType, version string) error
}

func NewMetaStorage(metaType string, meta config.Meta) (MetaStore, error) {
	switch metaType {
	case metaStoreMemory:
		return newMemoryMetaStore(), nil
	default:
		return nil, fmt.Errorf("unknow meta store type: %s", metaType)
	}
}
