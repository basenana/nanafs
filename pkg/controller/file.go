package controller

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/hyponet/eventbus/bus"
	"time"
)

type FileController interface {
	OpenFile(ctx context.Context, obj *types.Object, attr files.Attr) (files.File, error)
	ReadFile(ctx context.Context, file files.File, data []byte, offset int64) (n int, err error)
	WriteFile(ctx context.Context, file files.File, data []byte, offset int64) (n int64, err error)
	CloseFile(ctx context.Context, file files.File) error
	DeleteFileData(ctx context.Context, obj *types.Object) error
	IsStructured(obj *types.Object) bool
}

type OpenOption struct {
}

func (c *controller) OpenFile(ctx context.Context, obj *types.Object, attr files.Attr) (files.File, error) {
	defer utils.TraceRegion(ctx, "controller.openfile")()
	c.logger.Infow("open file", "file", obj.ID, "name", obj.Name, "attr", attr)
	if obj.IsGroup() {
		return nil, types.ErrIsGroup
	}
	if s := c.registry.GetSchema(obj.Kind); s != nil {
		attr.Meta = c.meta
		file, err := c.OpenStructuredObject(ctx, obj, &s, attr)
		if err != nil {
			c.logger.Errorw("open structured object failed", "err", err.Error())
			return nil, err
		}
		return file, c.SaveObject(ctx, file.GetObject())
	}

	var err error
	for dentry.IsMirrorObject(obj) {
		obj, err = c.meta.GetObject(ctx, obj.RefID)
		if err != nil {
			c.logger.Errorw("query source object error", "obj", obj.ID, "srcObj", obj.RefID, "err", err.Error())
			return nil, err
		}
		c.logger.Infow("replace source object", "srcObj", obj.ID)
	}

	if attr.Trunc {
		// TODO clean old data
		obj.Size = 0
	}

	file, err := files.Open(ctx, obj, attr)
	if err != nil {
		c.logger.Errorw("open file error", "obj", obj.ID, "err", err.Error())
		return nil, err
	}
	obj.AccessAt = time.Now()
	obj.ModifiedAt = time.Now()
	bus.Publish(fmt.Sprintf("object.file.%s.open", obj.ID), obj)
	return file, c.SaveObject(ctx, obj)
}

func (c *controller) ReadFile(ctx context.Context, file files.File, data []byte, offset int64) (n int, err error) {
	defer utils.TraceRegion(ctx, "controller.readfile")()
	n, err = file.Read(ctx, data, offset)
	if err != nil {
		return n, err
	}
	return n, nil
}

func (c *controller) WriteFile(ctx context.Context, file files.File, data []byte, offset int64) (n int64, err error) {
	defer utils.TraceRegion(ctx, "controller.writefile")()
	n, err = file.Write(ctx, data, offset)
	if err != nil {
		return n, err
	}
	obj := file.GetObject()
	obj.ModifiedAt = time.Now()
	return n, c.SaveObject(ctx, obj)
}

func (c *controller) CloseFile(ctx context.Context, file files.File) (err error) {
	defer utils.TraceRegion(ctx, "controller.closefile")()
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
	defer utils.TraceRegion(ctx, "controller.cleanfile")()
	c.logger.Infow("delete file", "file", obj.Name)
	err = c.storage.Delete(ctx, obj.ID)
	if err != nil {
		c.logger.Errorw("delete file error", "file", obj.ID, "err", err.Error())
		return err
	}
	bus.Publish(fmt.Sprintf("object.file.%s.delete", obj.ID), obj)
	return nil
}

func (c *controller) IsStructured(obj *types.Object) bool {
	if s := c.registry.GetSchema(obj.Kind); s != nil {
		return true
	}
	return false
}
