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
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"runtime/trace"
	"time"
)

func (c *controller) OpenFile(ctx context.Context, en dentry.Entry, attr dentry.Attr) (dentry.File, error) {
	defer trace.StartRegion(ctx, "controller.OpenFile").End()
	md := en.Metadata()
	c.logger.Debugw("open file", "file", md.ID, "name", md.Name, "attr", attr)
	if en.IsGroup() {
		return nil, types.ErrIsGroup
	}

	var err error
	for en.IsMirror() {
		var sourceEn dentry.Entry
		sourceEn, err = c.entry.GetEntry(ctx, md.RefID)
		if err != nil {
			c.logger.Errorw("query source object error", "entry", md.ID, "sourceEntry", md.RefID, "err", err.Error())
			return nil, err
		}
		en = sourceEn
		md = sourceEn.Metadata()
		c.logger.Infow("replace source object", "sourceEntry", md.ID)
	}

	if attr.Trunc {
		md.Size = 0
	}

	file, err := c.entry.Open(ctx, en, attr)
	if err != nil {
		c.logger.Errorw("open file error", "entry", md.ID, "err", err.Error())
		return nil, err
	}
	if attr.Write {
		md.ModifiedAt = time.Now()
	}
	md.AccessAt = time.Now()
	if err = c.SaveEntry(ctx, nil, en); err != nil {
		return nil, err
	}
	c.cache.putEntry(en)
	return file, nil
}

func (c *controller) ReadFile(ctx context.Context, file dentry.File, data []byte, offset int64) (n int64, err error) {
	defer trace.StartRegion(ctx, "controller.ReadFile").End()
	n, err = file.ReadAt(ctx, data, offset)
	if err != nil {
		return n, err
	}
	return n, nil
}

func (c *controller) WriteFile(ctx context.Context, file dentry.File, data []byte, offset int64) (n int64, err error) {
	defer trace.StartRegion(ctx, "controller.WriteFile").End()
	n, err = file.WriteAt(ctx, data, offset)
	if err != nil {
		return n, err
	}
	return n, nil
}

func (c *controller) CloseFile(ctx context.Context, file dentry.File) (err error) {
	defer trace.StartRegion(ctx, "controller.CloseFile").End()
	err = file.Close(ctx)
	if err != nil {
		c.logger.Errorw("close file error", "file", file.Metadata().ID, "err", err.Error())
		return err
	}
	en, err := c.GetEntry(ctx, file.Metadata().ID)
	if err != nil {
		c.logger.Errorw("query fresh entry error", "file", file.Metadata().ID, "err", err.Error())
		return err
	}
	c.cache.delEntry(en.Metadata().ID)
	return nil
}
