/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

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

type OpenOption struct {
}

func (c *controller) OpenFile(ctx context.Context, en dentry.Entry, attr files.Attr) (files.File, error) {
	defer utils.TraceRegion(ctx, "controller.openfile")()
	obj := en.Object()
	c.logger.Infow("open file", "file", obj.ID, "name", obj.Name, "attr", attr)
	if en.IsGroup() {
		return nil, types.ErrIsGroup
	}

	var err error
	for en.IsMirror() {
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
	if attr.Write {
		obj.ModifiedAt = time.Now()
	}
	obj.AccessAt = time.Now()
	bus.Publish(fmt.Sprintf("object.file.%d.open", obj.ID), obj)
	return file, c.SaveEntry(ctx, nil, dentry.BuildEntry(obj, c.meta))
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
	return n, c.SaveEntry(ctx, nil, dentry.BuildEntry(obj, c.meta))
}

func (c *controller) CloseFile(ctx context.Context, file files.File) (err error) {
	defer utils.TraceRegion(ctx, "controller.closefile")()
	c.logger.Infow("close file", "file", file.GetObject().ID)
	err = file.Close(ctx)
	if err != nil {
		c.logger.Errorw("close file error", "file", file.GetObject().ID, "err", err.Error())
		return err
	}
	bus.Publish(fmt.Sprintf("object.file.%d.close", file.GetObject().ID), file.GetObject())
	return nil
}

func (c *controller) DeleteFileData(ctx context.Context, en dentry.Entry) (err error) {
	defer utils.TraceRegion(ctx, "controller.cleanfile")()
	obj := en.Object()
	c.logger.Infow("delete file", "file", obj.Name)
	err = c.storage.Delete(ctx, obj.ID)
	if err != nil {
		c.logger.Errorw("delete file error", "file", obj.ID, "err", err.Error())
		return err
	}
	bus.Publish(fmt.Sprintf("object.file.%d.delete", obj.ID), obj)
	return nil
}
