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
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"sync"
	"sync/atomic"
)

type File interface {
	Entry

	GetAttr() Attr
	WriteAt(ctx context.Context, data []byte, off int64) (int64, error)
	ReadAt(ctx context.Context, dest []byte, off int64) (int64, error)
	Fsync(ctx context.Context) error
	Close(ctx context.Context) (err error)
}

type file struct {
	Entry

	reader bio.Reader
	writer bio.Writer

	ref  int32
	attr Attr
	mux  sync.Mutex
}

var _ File = &file{}

func (f *file) GetAttr() Attr {
	return f.attr
}

func (f *file) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	if !f.attr.Write || f.writer == nil {
		return 0, types.ErrUnsupported
	}
	return f.writer.WriteAt(ctx, data, off)
}

func (f *file) Fsync(ctx context.Context) error {
	if !f.attr.Write {
		return types.ErrUnsupported
	}
	return f.writer.Fsync(ctx)
}

func (f *file) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	if !f.attr.Read || f.reader == nil {
		return 0, types.ErrUnsupported
	}
	return f.reader.ReadAt(ctx, dest, off)
}

func (f *file) Close(ctx context.Context) (err error) {
	atomic.AddInt32(&f.ref, -1)
	return
}

type Attr struct {
	Read   bool
	Write  bool
	Create bool
	Trunc  bool
	Direct bool
	Meta   storage.Meta
}
