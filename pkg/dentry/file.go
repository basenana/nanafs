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
	"fmt"
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

func openFile(en Entry, attr Attr, store storage.ObjectStore, fileStorage storage.Storage) (File, error) {
	f := &file{
		Entry: en,
		attr:  attr,
	}
	if fileStorage == nil {
		return nil, fmt.Errorf("storage %s not found", en.Metadata().Storage)
	}
	f.reader = bio.NewChunkReader(en.Object(), store.(storage.ChunkStore), fileStorage)
	if attr.Write {
		f.writer = bio.NewChunkWriter(f.reader)
	}
	return f, nil

}

type symlink struct {
	Entry

	data []byte
	attr Attr
}

var _ File = &symlink{}

func (s *symlink) GetAttr() Attr {
	return s.attr
}

func (s *symlink) WriteAt(ctx context.Context, data []byte, offset int64) (n int64, err error) {
	size := offset + int64(len(data))

	if size > int64(len(s.data)) {
		newData := make([]byte, size)
		copy(newData, s.data)
		s.data = newData
	}

	n = int64(copy(s.data[offset:], data))
	s.Metadata().Size = size
	_ = s.Flush(ctx)
	return
}

func (s *symlink) ReadAt(ctx context.Context, data []byte, offset int64) (int64, error) {
	n := copy(data, s.data[offset:])
	return int64(n), nil
}

func (s *symlink) Fsync(ctx context.Context) error {
	return s.Flush(ctx)
}

func (s *symlink) Flush(ctx context.Context) (err error) {
	deviceInfo := s.data
	eData := s.ExtendData()
	eData.Symlink = string(deviceInfo)
	return nil
}

func (s *symlink) Close(ctx context.Context) (err error) {
	return s.Flush(ctx)
}

func openSymlink(en Entry, attr Attr) (File, error) {
	if en.Metadata().Kind != types.SymLinkKind {
		return nil, fmt.Errorf("not symlink")
	}

	var raw []byte
	eData := en.ExtendData()
	if eData.Symlink != "" {
		raw = []byte(eData.Symlink)
	}

	if raw == nil {
		raw = make([]byte, 0, 512)
	}

	return &symlink{Entry: en, data: raw, attr: attr}, nil
}

type Attr struct {
	Read   bool
	Write  bool
	Create bool
	Trunc  bool
	Direct bool
	Meta   storage.Meta
}
