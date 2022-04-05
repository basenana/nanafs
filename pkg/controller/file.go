package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/object"
)

type FileController interface {
	OpenFile(ctx context.Context, entry *dentry.Entry, attr files.Attr) (*files.File, error)
	WriteFile(ctx context.Context, file *files.File, data []byte, offset int64) (n int64, err error)
	CloseFile(ctx context.Context, file *files.File) error
	DeleteFileData(ctx context.Context, entry *dentry.Entry) error
}

type OpenOption struct {
}

func (c *controller) OpenFile(ctx context.Context, entry *dentry.Entry, attr files.Attr) (*files.File, error) {
	attr.Storage = c.storage
	if entry.IsGroup() {
		return nil, object.ErrIsGroup
	}
	return files.Open(ctx, entry, attr)
}

func (c *controller) WriteFile(ctx context.Context, file *files.File, data []byte, offset int64) (n int64, err error) {
	n, err = file.Write(ctx, data, offset)
	if err != nil {
		return n, err
	}
	entry := file.Object.(*dentry.Entry)
	return n, c.SaveEntry(ctx, entry)
}

func (c *controller) CloseFile(ctx context.Context, file *files.File) error {
	return file.Close(ctx)
}

func (c *controller) DeleteFileData(ctx context.Context, entry *dentry.Entry) error {
	return c.storage.Delete(ctx, entry.ID)
}
