package dentry

import (
	"context"
)

type DEntry struct {
	refCount uint32
}

type Cache interface {
	GetEntry(ctx context.Context, namespace string, path string) (*DEntry, error)
	FindEntry(ctx context.Context, namespace string, parentId int64, child string) (*DEntry, error)
	Invalid(entries ...int64)
}

type hashKey struct {
	parentID int64
	name     string
}
