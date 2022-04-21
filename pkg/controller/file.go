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
	ReadFile(ctx context.Context, file files.File, data []byte, offset int64) (n int, err error)
	WriteFile(ctx context.Context, file files.File, data []byte, offset int64) (n int64, err error)
	CloseFile(ctx context.Context, file files.File) error
	DeleteFileData(ctx context.Context, obj *types.Object) error
}

type OpenOption struct {
}

func (c *controller) OpenFile(ctx context.Context, obj *types.Object, attr files.Attr) (files.File, error) {
	c.logger.Infow("open file", "file", obj.ID, "name", obj.Name, "attr", attr)
	attr.Storage = c.storage
	attr.Meta = c.meta
	if obj.IsGroup() {
		return nil, types.ErrIsGroup
	}
	if s := c.registry.GetSchema(obj.Kind); s != nil {
		file, err := c.OpenStructuredObject(ctx, obj, &s, attr)
		if err != nil {
			c.logger.Errorw("open structured object failed", "err", err.Error())
			return nil, err
		}
		return file, c.SaveObject(ctx, file.GetObject())
	}
	file, err := files.Open(ctx, obj, attr)
	if err != nil {
		c.logger.Errorw("open file error", "obj", obj.ID, "err", err.Error())
		return nil, err
	}
	bus.Publish(fmt.Sprintf("object.file.%s.open", obj.ID), obj)
	return file, nil
}

func (c *controller) ReadFile(ctx context.Context, file files.File, data []byte, offset int64) (n int, err error) {
	c.logger.Infow("read file", "file", file.GetObject().ID, "name", file.GetObject().Name, "data", len(data), "offset", offset)
	n, err = file.Read(ctx, data, offset)
	if err != nil {
		return n, err
	}
	obj := file.GetObject()
	obj.AccessAt = time.Now()
	return n, c.SaveObject(ctx, obj)
}

func (c *controller) WriteFile(ctx context.Context, file files.File, data []byte, offset int64) (n int64, err error) {
	c.logger.Infow("write file", "file", file.GetObject().ID, "name", file.GetObject().Name, "data", len(data), "offset", offset)
	n, err = file.Write(ctx, data, offset)
	if err != nil {
		return n, err
	}
	obj := file.GetObject()
	obj.ModifiedAt = time.Now()
	return n, c.SaveObject(ctx, obj)
}

func (c *controller) CloseFile(ctx context.Context, file files.File) (err error) {
	c.logger.Infow("close file", "file", file.GetObject().ID)
	err = file.Close(ctx)
	if err != nil {
		c.logger.Errorw("close file error", "file", file.GetObject().ID, "err", err.Error())
		return err
	}
	bus.Publish(fmt.Sprintf("object.file.%s.close", file.GetObject().ID), file.GetObject())
	return nil
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
