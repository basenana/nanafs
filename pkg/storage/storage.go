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
	GetEntry(ctx context.Context, id string) (*types.Object, error)
	ListEntries(ctx context.Context, filter Filter) ([]*types.Object, error)
	SaveEntry(ctx context.Context, entry *types.Object) error
	DestroyEntry(ctx context.Context, entry *types.Object) error

	ListChildren(ctx context.Context, id string) (Iterator, error)
	ChangeParent(ctx context.Context, old *types.Object, parent *types.Object) error
}

func NewMetaStorage(metaType string, meta config.Meta) (MetaStore, error) {
	switch metaType {
	case metaStoreMemory:
		return newMemoryMetaStore(), nil
	default:
		return nil, fmt.Errorf("unknow meta store type: %s", metaType)
	}
}
