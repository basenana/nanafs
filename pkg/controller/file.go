package controller

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/hyponet/eventbus/bus"
	"time"
)

type FileController interface {
	OpenFile(ctx context.Context, obj *types.Object, attr files.Attr) (files.File, error)
	WriteFile(ctx context.Context, file files.File, data []byte, offset int64) (n int64, err error)
	CloseFile(ctx context.Context, file files.File) error
	ReadFile(ctx context.Context, file files.File, data []byte, offset int64) (n int, err error)
	DeleteFileData(ctx context.Context, obj *types.Object) error
}

type OpenOption struct {
}

func (c *controller) OpenFile(ctx context.Context, obj *types.Object, attr files.Attr) (files.File, error) {
	c.logger.Infow("open file", "obj", obj.Name, "attr", attr)
	attr.Storage = c.storage
	attr.Meta = c.meta
	if obj.IsGroup() {
		return nil, types.ErrIsGroup
	}
	if s := c.registry.GetSchema(obj.Kind); s != nil {
		return c.OpenStructuredObject(ctx, obj, s, attr)
	}
	return files.Open(ctx, obj, attr)
}

func (c *controller) WriteFile(ctx context.Context, file files.File, data []byte, offset int64) (n int64, err error) {
	c.logger.Infow("write file", "file", file.GetObject().Name)
	n, err = file.Write(ctx, data, offset)
	if err != nil {
		return n, err
	}
	obj := file.GetObject()
	return n, c.SaveObject(ctx, obj)
}

func (c *controller) CloseFile(ctx context.Context, file files.File) error {
	c.logger.Infow("close file", "file", file.GetObject().Name)
	return file.Close(ctx)
}

func (c *controller) DeleteFileData(ctx context.Context, obj *types.Object) (err error) {
	c.logger.Infow("delete file", "file", obj.Name)
	err = c.storage.Delete(ctx, obj.ID)
	if err != nil {
		c.logger.Errorw("delete file error", "file", obj.ID, "err", err.Error())
		return err
	}
	bus.Publish(fmt.Sprintf("object.file.%s.delete", obj.ID), obj)
	return nil
}

func (c *controller) ReadFile(ctx context.Context, file files.File, data []byte, offset int64) (n int, err error) {
	c.logger.Infow("read file", "file", file.GetObject().ID, "data", len(data), "offset", offset)
	n, err = file.Read(ctx, data, offset)
	if err != nil {
		return n, err
	}
	obj := file.GetObject()
	obj.AccessAt = time.Now()
	return n, c.SaveObject(ctx, obj)
}
