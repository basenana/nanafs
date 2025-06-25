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

package core

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/events"
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

type RawFile interface {
	GetAttr() types.OpenAttr
	WriteAt(ctx context.Context, data []byte, off int64) (int64, error)
	ReadAt(ctx context.Context, dest []byte, off int64) (int64, error)
	Fsync(ctx context.Context) error
	Flush(ctx context.Context) error
	Close(ctx context.Context) (err error)
}

type rawFile struct {
	namespace string
	entryID   int64
	size      int64

	reader bio.Reader
	writer bio.Writer

	attr types.OpenAttr
	cfg  *config.FS
	mux  sync.Mutex
}

var _ RawFile = &rawFile{}

func (f *rawFile) GetAttr() types.OpenAttr {
	return f.attr
}

func (f *rawFile) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	defer trace.StartRegion(ctx, "fs.core.file.WriteAt").End()
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

func (f *rawFile) Flush(ctx context.Context) error {
	defer trace.StartRegion(ctx, "fs.core.file.Flush").End()
	if !f.attr.Write {
		return nil
	}
	err := f.writer.Flush(ctx)
	if err != nil {
		fileEntryLogger.Errorw("flush file error", "entry", f.entryID, "err", err)
	}
	return err
}

func (f *rawFile) Fsync(ctx context.Context) error {
	defer trace.StartRegion(ctx, "fs.core.file.Fsync").End()
	if !f.attr.Write {
		return types.ErrUnsupported
	}
	err := f.writer.Flush(ctx)
	if err != nil {
		fileEntryLogger.Errorw("fsync file error", "entry", f.entryID, "err", err)
	}
	return err
}

func (f *rawFile) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	defer trace.StartRegion(ctx, "fs.core.file.ReadAt").End()
	if !f.attr.Read || f.reader == nil {
		return 0, types.ErrUnsupported
	}
	n, err := f.reader.ReadAt(ctx, dest, off)
	if err != nil && err != io.EOF {
		fileEntryLogger.Errorw("read file error", "entry", f.entryID, "off", off, "err", err)
	}
	return n, err
}

func (f *rawFile) Close(ctx context.Context) (err error) {
	defer trace.StartRegion(ctx, "fs.core.file.Close").End()
	defer publicEntryActionEvent(events.TopicNamespaceFile, events.ActionTypeClose, f.namespace, f.entryID)
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

func openFile(en *types.Entry, attr types.OpenAttr, chunkStore bio.ChunkStore, fileStorage storage.Storage) (RawFile, error) {
	f := &rawFile{namespace: en.Namespace, entryID: en.ID, size: en.Size, attr: attr}
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
	namespace string
	entryID   int64
	store     metastore.EntryStore

	size       int64
	modifiedAt time.Time
	data       []byte
	attr       types.OpenAttr
}

var _ RawFile = &symlink{}

func (s *symlink) GetAttr() types.OpenAttr {
	return s.attr
}

func (s *symlink) WriteAt(ctx context.Context, data []byte, off int64) (n int64, err error) {
	defer trace.StartRegion(ctx, "fs.core.symlink.WriteAt").End()
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
	defer trace.StartRegion(ctx, "fs.core.symlink.ReadAt").End()
	if s.data == nil || off >= s.size {
		return 0, io.EOF
	}
	return int64(copy(dest, s.data[off:s.size])), nil
}

func (s *symlink) Fsync(ctx context.Context) error {
	return s.Flush(ctx)
}

func (s *symlink) Flush(ctx context.Context) (err error) {
	defer trace.StartRegion(ctx, "fs.core.symlink.Flush").End()
	err = s.store.Flush(ctx, s.namespace, s.entryID, s.size)
	if err != nil {
		return err
	}

	sp := types.SymlinkProperties{Symlink: string(s.data[:s.size])}
	return s.store.UpdateEntryProperties(ctx, s.namespace, types.PropertyTypeSymlink, s.entryID, sp)
}

func (s *symlink) Close(ctx context.Context) error {
	defer trace.StartRegion(ctx, "fs.core.symlink.Close").End()
	defer publicEntryActionEvent(events.TopicNamespaceFile, events.ActionTypeClose, s.namespace, s.entryID)
	defer decreaseOpenedFile(s.entryID)
	return s.Flush(ctx)
}

func openSymlink(store metastore.Meta, en *types.Entry, attr types.OpenAttr) (RawFile, error) {
	if en.Kind != types.SymLinkKind {
		return nil, fmt.Errorf("not symlink")
	}

	var (
		raw  []byte
		size int64
		sp   = &types.SymlinkProperties{}
	)
	err := store.GetEntryProperties(context.TODO(), en.Namespace, types.PropertyTypeSymlink, en.ID, sp)
	if err != nil {
		return nil, logOperationError(fileOperationErrorCounter, "init", err)
	}
	if sp.Symlink != "" {
		raw = []byte(sp.Symlink)
		size = int64(len(raw))
	}

	if raw == nil {
		raw = make([]byte, 512)
	}

	increaseOpenedFile(en.ID)
	return &symlink{namespace: en.Namespace, entryID: en.ID, size: size, modifiedAt: en.ModifiedAt, store: store, data: raw, attr: attr}, nil
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
