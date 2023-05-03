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

package dentry

import (
	"context"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
)

type instrumentalFile struct {
	file File
}

func (i *instrumentalFile) Metadata() *types.Metadata {
	return i.file.Metadata()
}

func (i *instrumentalFile) GetExtendData(ctx context.Context) (types.ExtendData, error) {
	return i.file.GetExtendData(ctx)
}

func (i *instrumentalFile) UpdateExtendData(ctx context.Context, ed types.ExtendData) error {
	return i.file.UpdateExtendData(ctx, ed)
}

func (i *instrumentalFile) GetExtendField(ctx context.Context, fKey string) (*string, error) {
	return i.file.GetExtendField(ctx, fKey)
}

func (i *instrumentalFile) SetExtendField(ctx context.Context, fKey, fVal string) error {
	return i.file.SetExtendField(ctx, fKey, fVal)
}

func (i *instrumentalFile) RemoveExtendField(ctx context.Context, fKey string) error {
	return i.file.RemoveExtendField(ctx, fKey)
}

func (i *instrumentalFile) IsGroup() bool {
	return i.file.IsGroup()
}

func (i *instrumentalFile) IsMirror() bool {
	return i.file.IsMirror()
}

func (i *instrumentalFile) Group() Group {
	return i.file.Group()
}

func (i *instrumentalFile) GetAttr() Attr {
	return i.file.GetAttr()
}

func (i *instrumentalFile) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	return i.file.WriteAt(ctx, data, off)
}

func (i *instrumentalFile) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	return i.file.ReadAt(ctx, dest, off)
}

func (i *instrumentalFile) Fsync(ctx context.Context) error {
	return i.file.Fsync(ctx)
}

func (i *instrumentalFile) Flush(ctx context.Context) error {
	return i.file.Flush(ctx)
}

func (i *instrumentalFile) Close(ctx context.Context) (err error) {
	defer PublicFileActionEvent(events.ActionTypeClose, i.file)
	return i.file.Close(ctx)
}
