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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"go.uber.org/zap"
	"io"
	"runtime/trace"
	"sync"
	"time"
)

var (
	fileEntryLogger *zap.SugaredLogger
)

type File interface {
	Entry

	GetAttr() Attr
	WriteAt(ctx context.Context, data []byte, off int64) (int64, error)
	ReadAt(ctx context.Context, dest []byte, off int64) (int64, error)
	Fsync(ctx context.Context) error
	Flush(ctx context.Context) error
	Close(ctx context.Context) (err error)
}

type file struct {
	Entry

	reader bio.Reader
	writer bio.Writer

	attr  Attr
	reset CacheResetter
	cfg   *config.FS
	mux   sync.Mutex
}

var _ File = &file{}

func (f *file) GetAttr() Attr {
	return f.attr
}

func (f *file) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	defer trace.StartRegion(ctx, "dentry.file.WriteAt").End()
	if !f.attr.Write || f.writer == nil {
		return 0, types.ErrUnsupported
	}
	n, err := f.writer.WriteAt(ctx, data, off)
	if err != nil {
		fileEntryLogger.Errorw("write file error", "entry", f.Entry.Metadata().ID, "off", off, "err", err)
	}
	meta := f.Metadata()
	meta.ModifiedAt = time.Now()
	if meta.Size < off+n {
		meta.Size = off + n
	}
	if f.reset != nil {
		f.reset.ResetEntry(f)
	}
	return n, err
}

func (f *file) Flush(ctx context.Context) error {
	defer trace.StartRegion(ctx, "dentry.file.Flush").End()
	if !f.attr.Write {
		return nil
	}
	err := f.writer.Flush(ctx)
	if err != nil {
		fileEntryLogger.Errorw("flush file error", "entry", f.Entry.Metadata().ID, "err", err)
	}
	if f.reset != nil {
		f.reset.ResetEntry(f)
	}
	return err
}

func (f *file) Fsync(ctx context.Context) error {
	defer trace.StartRegion(ctx, "dentry.file.Fsync").End()
	if !f.attr.Write {
		return types.ErrUnsupported
	}
	err := f.writer.Flush(ctx)
	if err != nil {
		fileEntryLogger.Errorw("fsync file error", "entry", f.Entry.Metadata().ID, "err", err)
	}
	return err
}

func (f *file) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	defer trace.StartRegion(ctx, "dentry.file.ReadAt").End()
	if !f.attr.Read || f.reader == nil {
		return 0, types.ErrUnsupported
	}
	n, err := f.reader.ReadAt(ctx, dest, off)
	if err != nil && err != io.EOF {
		fileEntryLogger.Errorw("read file error", "entry", f.Entry.Metadata().ID, "off", off, "err", err)
	}
	return n, err
}

func (f *file) Close(ctx context.Context) (err error) {
	defer trace.StartRegion(ctx, "dentry.file.Close").End()
	defer decreaseOpenedFile(f.Metadata().ID)
	defer f.reader.Close()
	if f.attr.Write {
		defer f.writer.Close()
		if f.cfg.Writeback {
			return f.writer.Flush(ctx)
		}
		return f.writer.Fsync(ctx)
	}
	return nil
}

func openFile(en Entry, attr Attr, store metastore.ObjectStore, fileStorage storage.Storage, cacheReset CacheResetter, cfg *config.FS) (File, error) {
	f := &file{
		Entry: en,
		attr:  attr,
		reset: cacheReset,
		cfg:   cfg,
	}
	if fileStorage == nil {
		return nil, fmt.Errorf("storage %s not found", en.Metadata().Storage)
	}
	f.reader = bio.NewChunkReader(en.Metadata(), store.(metastore.ChunkStore), fileStorage)
	if attr.Write {
		f.writer = bio.NewChunkWriter(f.reader)
	}
	increaseOpenedFile(en.Metadata().ID)
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
	defer trace.StartRegion(ctx, "dentry.symlink.WriteAt").End()
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
	defer trace.StartRegion(ctx, "dentry.symlink.ReadAt").End()
	n := copy(data, s.data[offset:])
	return int64(n), nil
}

func (s *symlink) Fsync(ctx context.Context) error {
	return s.Flush(ctx)
}

func (s *symlink) Flush(ctx context.Context) (err error) {
	defer trace.StartRegion(ctx, "dentry.symlink.Flush").End()
	deviceInfo := s.data
	eData, err := s.GetExtendData(ctx)
	if err != nil {
		return err
	}
	eData.Symlink = string(deviceInfo)
	return s.UpdateExtendData(ctx, eData)
}

func (s *symlink) Close(ctx context.Context) (err error) {
	defer trace.StartRegion(ctx, "dentry.symlink.Close").End()
	defer decreaseOpenedFile(s.Metadata().ID)
	return s.Flush(ctx)
}

func openSymlink(en Entry, attr Attr) (File, error) {
	if en.Metadata().Kind != types.SymLinkKind {
		return nil, fmt.Errorf("not symlink")
	}

	var raw []byte
	eData, err := en.GetExtendData(context.TODO())
	if err != nil {
		return nil, err
	}
	if eData.Symlink != "" {
		raw = []byte(eData.Symlink)
	}

	if raw == nil {
		raw = make([]byte, 0, 512)
	}

	increaseOpenedFile(en.Metadata().ID)
	return &symlink{Entry: en, data: raw, attr: attr}, nil
}

type Attr struct {
	Read   bool
	Write  bool
	Create bool
	Trunc  bool
	Direct bool
}

var (
	openedFiles       = map[int64]int{}
	openedFileMapLock sync.Mutex
)

func increaseOpenedFile(fid int64) {
	openedFileMapLock.Lock()
	openedFiles[fid]++
	openedFileMapLock.Unlock()
}

func decreaseOpenedFile(fid int64) {
	openedFileMapLock.Lock()
	openedFiles[fid]--
	openedFileMapLock.Unlock()
}

func isFileOpened(fid int64) bool {
	openedFileMapLock.Lock()
	count := openedFiles[fid]
	openedFileMapLock.Unlock()
	return count > 0
}
