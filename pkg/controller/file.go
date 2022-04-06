package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
)

type FileController interface {
	OpenFile(ctx context.Context, obj *types.Object, attr files.Attr) (*files.File, error)
	WriteFile(ctx context.Context, file *files.File, data []byte, offset int64) (n int64, err error)
	CloseFile(ctx context.Context, file *files.File) error
	DeleteFileData(ctx context.Context, obj *types.Object) error
}

type OpenOption struct {
}

func (c *controller) OpenFile(ctx context.Context, obj *types.Object, attr files.Attr) (*files.File, error) {
	c.logger.Infow("open file", "obj", obj.Name, "attr", attr)
	attr.Storage = c.storage
	if obj.IsGroup() {
		return nil, types.ErrIsGroup
	}
	return files.Open(ctx, obj, attr)
}

func (c *controller) WriteFile(ctx context.Context, file *files.File, data []byte, offset int64) (n int64, err error) {
	meta := file.GetObjectMeta()
	c.logger.Infow("write file", "file", meta.Name)
	n, err = file.Write(ctx, data, offset)
	if err != nil {
		return n, err
	}
	obj := file.Object
	return n, c.SaveObject(ctx, obj)
}

func (c *controller) CloseFile(ctx context.Context, file *files.File) error {
	meta := file.GetObjectMeta()
	c.logger.Infow("close file", "file", meta.Name)
	return file.Close(ctx)
}

func (c *controller) DeleteFileData(ctx context.Context, obj *types.Object) error {
	c.logger.Infow("delete file", "file", obj.Name)
	return c.storage.Delete(ctx, obj.ID)
}
