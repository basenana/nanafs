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
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
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
	GetAttr() types.OpenAttr
	WriteAt(ctx context.Context, data []byte, off int64) (int64, error)
	ReadAt(ctx context.Context, dest []byte, off int64) (int64, error)
	Fsync(ctx context.Context) error
	Flush(ctx context.Context) error
	Close(ctx context.Context) (err error)
}

type file struct {
	entryID int64
	size    int64

	reader bio.Reader
	writer bio.Writer

	attr types.OpenAttr
	cfg  *config.FS
	mux  sync.Mutex
}

var _ File = &file{}

func (f *file) GetAttr() types.OpenAttr {
	return f.attr
}

func (f *file) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	defer trace.StartRegion(ctx, "dentry.file.WriteAt").End()
	if !f.attr.Write || f.writer == nil {
		return 0, types.ErrUnsupported
	}
	n, err := f.writer.WriteAt(ctx, data, off)
	if err != nil {
		fileEntryLogger.Errorw("write file error", "entry", f.entryID, "off", off, "err", err)
	}
	if f.size < off+n {
		f.size = off + n
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
		fileEntryLogger.Errorw("flush file error", "entry", f.entryID, "err", err)
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
		fileEntryLogger.Errorw("fsync file error", "entry", f.entryID, "err", err)
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
		fileEntryLogger.Errorw("read file error", "entry", f.entryID, "off", off, "err", err)
	}
	return n, err
}

func (f *file) Close(ctx context.Context) (err error) {
	defer trace.StartRegion(ctx, "dentry.file.Close").End()
	// TODO: fix close file event
	//defer PublicFileActionEvent(events.ActionTypeClose, f.entry)
	defer decreaseOpenedFile(f.entryID)
	defer f.reader.Close()
	if f.attr.Write {
		defer f.writer.Close()
		if f.attr.FsWriteback {
			return f.writer.Flush(ctx)
		}
		return f.writer.Fsync(ctx)
	}
	return nil
}

func openFile(en *types.Metadata, attr types.OpenAttr, chunkStore metastore.ChunkStore, fileStorage storage.Storage) (File, error) {
	f := &file{entryID: en.ID, size: en.Size, attr: attr}
	if fileStorage == nil {
		return nil, logOperationError(fileOperationErrorCounter, "init", fmt.Errorf("storage %s not found", en.Storage))
	}
	f.reader = bio.NewChunkReader(en, chunkStore, fileStorage)
	if attr.Write {
		f.writer = bio.NewChunkWriter(f.reader)
	}
	increaseOpenedFile(en.ID)
	return f, nil
}

type symlink struct {
	entryID int64
	mgr     *manager
	store   metastore.DEntry

	plugin.MemFS
	size       int64
	modifiedAt time.Time
	data       []byte
	attr       types.OpenAttr
}

var _ File = &symlink{}

func (s *symlink) GetAttr() types.OpenAttr {
	return s.attr
}

func (s *symlink) WriteAt(ctx context.Context, data []byte, off int64) (n int64, err error) {
	defer trace.StartRegion(ctx, "dentry.symlink.WriteAt").End()
	newSize := off + int64(len(data))
	if off+int64(len(data)) > int64(len(s.data)) {
		blk := make([]byte, newSize)
		copy(blk, s.data[:s.size])
		s.data = blk
	}
	n = int64(copy(s.data[off:], data))
	if newSize > s.size {
		s.size = newSize
	}
	s.modifiedAt = time.Now()
	_ = s.Flush(ctx)
	return
}

func (s *symlink) ReadAt(ctx context.Context, dest []byte, off int64) (n int64, err error) {
	defer trace.StartRegion(ctx, "dentry.symlink.ReadAt").End()
	if s.data == nil || off > s.size {
		return 0, io.EOF
	}
	return int64(copy(dest, s.data[off:s.size])), nil
}

func (s *symlink) Fsync(ctx context.Context) error {
	return s.Flush(ctx)
}

func (s *symlink) Flush(ctx context.Context) (err error) {
	defer trace.StartRegion(ctx, "dentry.symlink.Flush").End()
	err = s.mgr.store.Flush(ctx, s.entryID, s.size)
	if err != nil {
		return err
	}

	eData, err := s.mgr.GetEntryExtendData(ctx, s.entryID)
	if err != nil {
		return err
	}
	eData.Symlink = string(s.data[:s.size])
	return s.mgr.UpdateEntryExtendData(ctx, s.entryID, eData)
}

func (s *symlink) Close(ctx context.Context) error {
	defer trace.StartRegion(ctx, "dentry.symlink.Close").End()
	defer s.mgr.publicEntryActionEvent(events.TopicNamespaceFile, events.ActionTypeClose, s.entryID)
	defer decreaseOpenedFile(s.entryID)
	return s.Flush(ctx)
}

func openSymlink(mgr *manager, en *types.Metadata, attr types.OpenAttr) (File, error) {
	if en.Kind != types.SymLinkKind {
		return nil, fmt.Errorf("not symlink")
	}

	var (
		raw  []byte
		size int64
	)
	eData, err := mgr.GetEntryExtendData(context.TODO(), en.ID)
	if err != nil {
		return nil, logOperationError(fileOperationErrorCounter, "init", err)
	}
	if eData.Symlink != "" {
		raw = []byte(eData.Symlink)
		size = int64(len(raw))
	}

	if raw == nil {
		raw = make([]byte, 512)
	}

	increaseOpenedFile(en.ID)
	return &symlink{entryID: en.ID, size: size, modifiedAt: en.ModifiedAt, mgr: mgr, data: raw, attr: attr}, nil
}

type extFile struct {
	pluginapi.File
	attr    types.OpenAttr
	entryID int64
}

func (e *extFile) GetAttr() types.OpenAttr {
	return e.attr
}

func (e *extFile) Flush(ctx context.Context) error {
	return e.Fsync(ctx)
}

func openExternalFile(ctx context.Context, en *StubEntry, p plugin.MirrorPlugin, attr types.OpenAttr) (File, error) {
	f, err := p.Open(ctx, en.path)
	if err != nil {
		return nil, err
	}
	eFile := &extFile{
		File:    f,
		attr:    attr,
		entryID: en.id,
	}
	if attr.Trunc || en.info.Size == 0 {
		err = f.Trunc(ctx)
		if err != nil {
			return nil, err
		}
	}
	return eFile, nil
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
