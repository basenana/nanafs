package controller

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/hyponet/eventbus/bus"
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
	file, err := files.Open(ctx, obj, attr)
	if err != nil {
		return nil, err
	}
	bus.Publish(fmt.Sprintf("object.file.%s.open", obj.ID), obj)
	return file, nil
}

func (c *controller) WriteFile(ctx context.Context, file *files.File, data []byte, offset int64) (n int64, err error) {
	c.logger.Infow("write file", "file", file.Object.Name)
	n, err = file.Write(ctx, data, offset)
	if err != nil {
		return n, err
	}
	obj := file.Object
	return n, c.SaveObject(ctx, obj)
}

func (c *controller) CloseFile(ctx context.Context, file *files.File) (err error) {
	c.logger.Infow("close file", "file", file.Object.Name)
	err = file.Close(ctx)
	if err == nil {
		bus.Publish(fmt.Sprintf("object.file.%s.close", file.ID), file.Object)
	}
	return err
}

func (c *controller) DeleteFileData(ctx context.Context, obj *types.Object) (err error) {
	c.logger.Infow("delete file", "file", obj.Name)
	err = c.storage.Delete(ctx, obj.ID)
	if err == nil {
		bus.Publish(fmt.Sprintf("object.file.%s.delete", obj.ID), obj)
	}
	return err
}
