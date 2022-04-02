package storage

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"io"
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

	default:
		return nil, fmt.Errorf("unknow storage id: %s", storageID)
	}
}

type MetaStore interface {
	GetEntry(ctx context.Context, id string) (*dentry.Entry, error)
	ListEntries(ctx context.Context, filter Filter) ([]*dentry.Entry, error)
	SaveEntry(ctx context.Context, entry *dentry.Entry) error
	DestroyEntry(ctx context.Context, entry *dentry.Entry) error

	ListChildren(ctx context.Context, id string) (Iterator, error)
	ChangeParent(ctx context.Context, old *dentry.Entry, parent *dentry.Entry) error
}

func NewMetaStorage(metaType string, meta config.Meta) (MetaStore, error) {
	switch metaType {
	case metaStoreMemory:
		return newMemoryMetaStore(), nil
	default:
		return nil, fmt.Errorf("unknow meta store type: %s", metaType)
	}
}
