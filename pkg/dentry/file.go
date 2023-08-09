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
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
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
	GetAttr() Attr
	WriteAt(ctx context.Context, data []byte, off int64) (int64, error)
	ReadAt(ctx context.Context, dest []byte, off int64) (int64, error)
	Fsync(ctx context.Context) error
	Flush(ctx context.Context) error
	Close(ctx context.Context) (err error)
}

type file struct {
	meta *types.Metadata

	reader bio.Reader
	writer bio.Writer

	attr Attr
	cfg  *config.FS
	mux  sync.Mutex
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
		fileEntryLogger.Errorw("write file error", "entry", f.meta.ID, "off", off, "err", err)
	}
	f.meta.ModifiedAt = time.Now()
	if f.meta.Size < off+n {
		f.meta.Size = off + n
	}
	cacheStore.delEntryCache(f.meta.ID)
	return n, err
}

func (f *file) Flush(ctx context.Context) error {
	defer trace.StartRegion(ctx, "dentry.file.Flush").End()
	if !f.attr.Write {
		return nil
	}
	err := f.writer.Flush(ctx)
	if err != nil {
		fileEntryLogger.Errorw("flush file error", "entry", f.meta.ID, "err", err)
	}
	cacheStore.delEntryCache(f.meta.ID)
	return err
}

func (f *file) Fsync(ctx context.Context) error {
	defer trace.StartRegion(ctx, "dentry.file.Fsync").End()
	if !f.attr.Write {
		return types.ErrUnsupported
	}
	err := f.writer.Flush(ctx)
	if err != nil {
		fileEntryLogger.Errorw("fsync file error", "entry", f.meta.ID, "err", err)
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
		fileEntryLogger.Errorw("read file error", "entry", f.meta.ID, "off", off, "err", err)
	}
	return n, err
}

func (f *file) Close(ctx context.Context) (err error) {
	defer trace.StartRegion(ctx, "dentry.file.Close").End()
	defer PublicFileActionEvent(events.ActionTypeClose, f.meta)
	defer decreaseOpenedFile(f.meta.ID)
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

func openFile(en *types.Metadata, attr Attr, store metastore.ObjectStore, fileStorage storage.Storage, cfg *config.FS) (File, error) {
	f := &file{meta: en, attr: attr, cfg: cfg}
	if fileStorage == nil {
		return nil, logOperationError(fileOperationErrorCounter, "init", fmt.Errorf("storage %s not found", en.Storage))
	}
	f.reader = bio.NewChunkReader(en, store.(metastore.ChunkStore), fileStorage)
	if attr.Write {
		f.writer = bio.NewChunkWriter(f.reader)
	}
	increaseOpenedFile(en.ID)
	return f, nil
}

type symlink struct {
	meta *types.Metadata
	mgr  Manager

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
	s.meta.Size = size
	s.meta.ModifiedAt = time.Now()
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
	eData, err := s.mgr.GetEntryExtendData(ctx, s.meta.ID)
	if err != nil {
		return err
	}
	eData.Symlink = string(deviceInfo)
	return s.mgr.UpdateEntryExtendData(ctx, s.meta.ID, eData)
}

func (s *symlink) Close(ctx context.Context) (err error) {
	defer trace.StartRegion(ctx, "dentry.symlink.Close").End()
	defer PublicFileActionEvent(events.ActionTypeClose, s.meta)
	defer decreaseOpenedFile(s.meta.ID)
	return s.Flush(ctx)
}

func openSymlink(mgr Manager, en *types.Metadata, attr Attr) (File, error) {
	if en.Kind != types.SymLinkKind {
		return nil, fmt.Errorf("not symlink")
	}

	var raw []byte
	eData, err := mgr.GetEntryExtendData(context.TODO(), en.ID)
	if err != nil {
		return nil, logOperationError(fileOperationErrorCounter, "init", err)
	}
	if eData.Symlink != "" {
		raw = []byte(eData.Symlink)
	}

	if raw == nil {
		raw = make([]byte, 0, 512)
	}

	increaseOpenedFile(en.ID)
	return &symlink{meta: en, data: raw, attr: attr}, nil
}

type extFile struct {
	meta  *types.Metadata
	attr  Attr
	cfg   *config.FS
	size  int64
	store metastore.ObjectStore
	stub  plugin.MirrorPlugin
}

func (e *extFile) GetAttr() Attr {
	return e.attr
}

func (e *extFile) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	n, err := e.stub.WriteAt(ctx, data, off)
	if off+n > e.size {
		e.size = off + n
	}
	e.meta.ModifiedAt = time.Now()
	return n, err
}

func (e *extFile) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	return e.stub.ReadAt(ctx, dest, off)
}

func (e *extFile) Fsync(ctx context.Context) error {
	if e.meta.Size < e.size {
		err := cacheStore.patchEntryMeta(ctx, &types.Metadata{Size: e.size, ModifiedAt: e.meta.ModifiedAt})
		if err != nil {
			return err
		}
	}
	return e.stub.Fsync(ctx)
}

func (e *extFile) Flush(ctx context.Context) error {
	return e.stub.Fsync(ctx)
}

func (e *extFile) Close(ctx context.Context) error {
	err := e.Fsync(ctx)
	if err != nil {
		return err
	}
	return e.stub.Close(ctx)
}

func openExternalFile(en *types.Metadata, ps *types.PlugScope, attr Attr, store metastore.ObjectStore, cfg *config.FS) (File, error) {
	if ps == nil {
		return nil, fmt.Errorf("extend entry has no plug scop")
	}
	stub, err := plugin.NewMirrorPlugin(context.TODO(), *ps)
	if err != nil {
		return nil, fmt.Errorf("build mirror plugin failed: %s", err)
	}
	eFile := &extFile{meta: en, attr: attr, size: en.Size, store: store, cfg: cfg, stub: stub}
	if attr.Trunc || en.Size == 0 {
		err = stub.Trunc(context.TODO())
		if err != nil {
			return nil, err
		}
		en.Size = 0
		eFile.size = 0
	}
	return eFile, nil
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

func IsFileOpened(fid int64) bool {
	openedFileMapLock.Lock()
	count := openedFiles[fid]
	openedFileMapLock.Unlock()
	return count > 0
}
