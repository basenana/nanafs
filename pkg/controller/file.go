package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/files"
)

type FileController interface {
	OpenFile(ctx context.Context, entry *dentry.Entry, attr files.Attr) (*files.File, error)
	CloseFile(ctx context.Context, file *files.File) error
	DeleteFileData(ctx context.Context, entry *dentry.Entry) error
}

type OpenOption struct {
}

func (c *controller) OpenFile(ctx context.Context, entry *dentry.Entry, attr files.Attr) (*files.File, error) {
	attr.Storage = c.storage
	return files.Open(ctx, entry, attr)
}

func (c *controller) CloseFile(ctx context.Context, file *files.File) error {
	return file.Close(ctx)
}

func (c *controller) DeleteFileData(ctx context.Context, entry *dentry.Entry) error {
	return c.storage.Delete(ctx, entry.ID)
}
