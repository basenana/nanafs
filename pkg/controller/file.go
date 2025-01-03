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
	"io"
	"runtime/trace"
)

func (c *controller) OpenFile(ctx context.Context, entryID int64, attr types.OpenAttr) (dentry.File, error) {
	defer trace.StartRegion(ctx, "controller.OpenFile").End()
	entry, err := c.GetEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}
	c.logger.Debugw("open file", "file", entry.ID, "name", entry.Name, "attr", attr)
	if entry.IsGroup {
		return nil, types.ErrIsGroup
	}

	for types.IsMirrored(entry) {
		var sourceEn *types.Entry
		sourceEn, err = c.entry.GetEntry(ctx, entry.RefID)
		if err != nil {
			c.logger.Errorw("query source object error", "entry", entry.ID, "sourceEntry", entry.RefID, "err", err)
			return nil, err
		}
		entry = sourceEn
		c.logger.Infow("replace source entry", "sourceEntry", entry.ID)
	}

	file, err := c.entry.Open(ctx, entry.ID, attr)
	if err != nil {
		c.logger.Errorw("open file error", "entry", entry.ID, "err", err.Error())
		return nil, err
	}
	return file, nil
}

func (c *controller) ReadFile(ctx context.Context, file dentry.File, data []byte, offset int64) (n int64, err error) {
	defer trace.StartRegion(ctx, "controller.ReadFile").End()
	n, err = file.ReadAt(ctx, data, offset)
	if err != nil && err != io.EOF {
		c.logger.Errorw("read file failed", "offset", offset, "file", file.GetAttr().EntryID, "err", err)
		return n, err
	}
	return n, err
}

func (c *controller) WriteFile(ctx context.Context, file dentry.File, data []byte, offset int64) (n int64, err error) {
	defer trace.StartRegion(ctx, "controller.WriteFile").End()
	n, err = file.WriteAt(ctx, data, offset)
	if err != nil {
		c.logger.Errorw("write file failed", "offset", offset, "file", file.GetAttr().EntryID, "err", err)
		return n, err
	}
	return n, nil
}

func (c *controller) CloseFile(ctx context.Context, file dentry.File) (err error) {
	defer trace.StartRegion(ctx, "controller.CloseFile").End()
	err = file.Close(ctx)
	if err != nil {
		c.logger.Errorw("close file failed", "file", file.GetAttr().EntryID, "err", err)
		return err
	}
	return nil
}
