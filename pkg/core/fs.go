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
	"errors"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"io/fs"
	"math"
	"runtime/trace"
	"strings"
	"time"
)

const (
	defaultFsMaxSize  = 1125899906842624
	fileNameMaxLength = 255
)

type FileSystem struct {
	core      Core
	store     metastore.Meta
	namespace string
	logger    *zap.SugaredLogger
}

func NewFileSystem(core Core, store metastore.Meta, namespace string) (*FileSystem, error) {
	err := core.CreateNamespace(context.Background(), namespace)
	if err != nil {
		return nil, err
	}
	return &FileSystem{
		core:      core,
		store:     store,
		namespace: namespace,
		logger:    logger.NewLogger("core.fs"),
	}, nil
}

func (f *FileSystem) FsInfo(ctx context.Context) Info {
	defer trace.StartRegion(ctx, "fs.FsInfo").End()

	nowTime := time.Now()
	if fsInfoCache != nil && nowTime.Before(fsInfoNextFetchAt) {
		return *fsInfoCache
	}

	info := Info{
		AvailInodes: math.MaxUint32,
		MaxSize:     defaultFsMaxSize,
	}

	sysInfo, err := f.store.SystemInfo(ctx)
	if err != nil {
		return info
	}

	info.Objects = uint64(sysInfo.ObjectCount)
	info.UsageSize = uint64(sysInfo.FileSizeTotal)

	fsInfoCache = &info
	fsInfoNextFetchAt.Add(time.Minute * 5)
	return info
}

func (f *FileSystem) Root(ctx context.Context) (*types.Entry, error) {
	return f.core.NamespaceRoot(ctx, f.namespace)
}

func (f *FileSystem) GetEntry(ctx context.Context, id int64) (*types.Entry, error) {
	return f.core.GetEntry(ctx, f.namespace, id)
}

func (f *FileSystem) GetEntryByPath(ctx context.Context, path string) (*types.Entry, *types.Entry, error) {
	var (
		crt, parent *types.Entry
		err         error
	)
	parent, err = f.Root(ctx)
	if err != nil {
		return nil, nil, err
	}

	entries := strings.Split(path, "/")
	for _, entryName := range entries {
		if entryName == "" {
			continue
		}

		if crt != nil {
			parent = crt
		}

		crt, err = f.LookUpEntry(ctx, parent.ID, entryName)
		if err != nil {
			return nil, nil, err
		}
	}

	return parent, crt, nil
}

func (f *FileSystem) LookUpEntry(ctx context.Context, parent int64, name string) (*types.Entry, error) {
	group, err := f.core.ListChildren(ctx, f.namespace, parent)
	if err != nil {
		return nil, err
	}
	for _, child := range group {
		en, err := f.core.GetEntry(ctx, f.namespace, child.ChildID)
		if err != nil {
			return nil, err
		}
		if en.Name == name {
			return en, nil
		}
	}
	return nil, types.ErrNotFound
}

func (f *FileSystem) CreateEntry(ctx context.Context, parent int64, attr types.EntryAttr) (*types.Entry, error) {
	if len(attr.Name) > fileNameMaxLength {
		return nil, types.ErrNameTooLong
	}
	return f.core.CreateEntry(ctx, f.namespace, parent, attr)
}

func (f *FileSystem) UpdateEntry(ctx context.Context, id int64, update types.UpdateEntry) (*types.Entry, error) {
	return f.core.UpdateEntry(ctx, f.namespace, id, update)
}

// MARK: tree

func (f *FileSystem) LinkEntry(ctx context.Context, srcEntryId, dstParentId int64, newEn types.EntryAttr) (*types.Entry, error) {
	if len(newEn.Name) > fileNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	oldEntry, err := f.LookUpEntry(ctx, dstParentId, newEn.Name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, err
	}
	if oldEntry != nil {
		return nil, types.ErrIsExist
	}

	return f.core.MirrorEntry(ctx, f.namespace, srcEntryId, dstParentId, newEn)
}

func (f *FileSystem) UnlinkEntry(ctx context.Context, parentID int64, child string, attr types.DestroyEntryAttr) error {
	parent, err := f.core.GetEntry(ctx, f.namespace, parentID)
	if err != nil {
		return err
	}
	if err = IsAccess(parent.Access, attr.Uid, attr.Gid, 0x2); err != nil {
		return types.ErrNoAccess
	}

	en, err := f.LookUpEntry(ctx, parent.ID, child)
	if err != nil {
		return err
	}
	if attr.Uid != 0 && attr.Uid != en.Access.UID && attr.Uid != parent.Access.UID && parent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoAccess
	}

	f.logger.Debugw("delete entry", "parent", parentID, "entry", child)
	return f.core.RemoveEntry(ctx, f.namespace, parentID, en.ID)
}

func (f *FileSystem) RmGroup(ctx context.Context, parentID int64, child string, attr types.DestroyEntryAttr) error {
	parent, err := f.core.GetEntry(ctx, f.namespace, parentID)
	if err != nil {
		return err
	}
	if err = IsAccess(parent.Access, attr.Uid, attr.Gid, 0x2); err != nil {
		return types.ErrNoAccess
	}

	en, err := f.LookUpEntry(ctx, parent.ID, child)
	if err != nil {
		return err
	}
	if attr.Uid != 0 && attr.Uid != en.Access.UID && attr.Uid != parent.Access.UID && parent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoAccess
	}

	if !en.IsGroup {
		return types.ErrNoGroup
	}

	children, err := f.core.ListChildren(ctx, f.namespace, en.ID)
	if err != nil {
		return err
	}

	if len(children) > 0 {
		return types.ErrNotEmpty
	}

	f.logger.Debugw("delete group", "parent", parentID, "entry", child)
	return f.core.RemoveEntry(ctx, f.namespace, parentID, en.ID)
}

func (f *FileSystem) Rename(ctx context.Context, targetId, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error {
	if len(newName) > fileNameMaxLength {
		return types.ErrNameTooLong
	}

	// need source dir WRITE
	oldParent, err := f.core.GetEntry(ctx, f.namespace, oldParentId)
	if err != nil {
		return err
	}
	if err = IsAccess(oldParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}
	// need dst dir WRITE
	newParent, err := f.core.GetEntry(ctx, f.namespace, newParentId)
	if err != nil {
		return err
	}
	if err = IsAccess(newParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}

	target, err := f.core.GetEntry(ctx, f.namespace, targetId)
	if err != nil {
		return err
	}
	if opt.Uid != 0 && opt.Uid != oldParent.Access.UID && opt.Uid != target.Access.UID && oldParent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoPerm
	}

	var existObjId *int64
	existObj, err := f.LookUpEntry(ctx, newParentId, newName)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
			return err
		}
	}

	if existObj != nil {
		if opt.Uid != 0 && opt.Uid != newParent.Access.UID && opt.Uid != existObj.Access.UID && newParent.Access.HasPerm(types.PermSticky) {
			return types.ErrNoPerm
		}
		eid := existObj.ID
		existObjId = &eid
	}

	return f.core.ChangeEntryParent(ctx, f.namespace, targetId, existObjId, oldParentId, newParentId, newName, opt)
}

// MARK: xattr

func (f *FileSystem) GetXAttr(ctx context.Context, id int64, fKey string) ([]byte, error) {
	return f.core.GetXAttr(ctx, f.namespace, id, fKey)
}

func (f *FileSystem) SetXAttr(ctx context.Context, id int64, fKey string, fVal []byte) error {
	return f.core.SetXAttr(ctx, f.namespace, id, fKey, fVal)
}

func (f *FileSystem) RemoveXAttr(ctx context.Context, id int64, fKey string) error {
	return f.core.RemoveXAttr(ctx, f.namespace, id, fKey)
}

// MARK: file

func (f *FileSystem) Open(ctx context.Context, id int64, attr types.OpenAttr) (File, error) {
	en, err := f.core.GetEntry(ctx, f.namespace, id)
	if err != nil {
		return nil, err
	}
	raw, err := f.core.Open(ctx, f.namespace, id, attr)
	if err != nil {
		return nil, err
	}
	return newFile(ctx, en, raw), nil
}

func (f *FileSystem) ListChildren(ctx context.Context, id int64) ([]*types.Entry, error) {
	children, err := f.core.ListChildren(ctx, f.namespace, id)
	if err != nil {
		return nil, err
	}
	result := make([]*types.Entry, 0, len(children))
	for _, child := range children {
		c, err := f.core.GetEntry(ctx, f.namespace, child.ChildID)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}

	return result, nil
}

func (f *FileSystem) OpenDir(ctx context.Context, id int64) (File, error) {
	en, err := f.core.GetEntry(ctx, f.namespace, id)
	if err != nil {
		return nil, err
	}
	if !en.IsGroup {
		return nil, types.ErrNoGroup
	}
	return newDIR(ctx, en, f), nil
}

type Info struct {
	Objects     uint64
	FileCount   uint64
	AvailInodes uint64
	MaxSize     uint64
	UsageSize   uint64
}

type FileInfo interface {
	ID() int64
	fs.FileInfo
}

type fInfo struct {
	*types.Entry
}

func (f *fInfo) ID() int64 {
	return f.Entry.ID
}

func (f *fInfo) Name() string {
	return f.Entry.Name
}

func (f *fInfo) Size() int64 {
	return f.Entry.Size
}

func (f *fInfo) Mode() fs.FileMode {
	return fs.FileMode(modeFromFileKind(f.Entry.Kind) | Access2Mode(f.Entry.Access))
}

func (f *fInfo) ModTime() time.Time {
	return f.Entry.ChangedAt
}

func (f *fInfo) IsDir() bool {
	return f.Entry.IsGroup
}

func (f *fInfo) Sys() any {
	return nil
}

var _ FileInfo = &fInfo{}

type File interface {
	io.ReadWriteCloser
	io.Seeker
	Readdir(count int) ([]FileInfo, error)
	Stat() (FileInfo, error)
	Raw() RawFile
}

type fsFile struct {
	ctx  context.Context
	info FileInfo
	raw  RawFile
	off  int64
}

func newFile(ctx context.Context, entry *types.Entry, raw RawFile) *fsFile {
	return &fsFile{
		ctx:  ctx,
		info: &fInfo{Entry: entry},
		raw:  raw,
	}
}

func (f *fsFile) Read(p []byte) (n int, err error) {
	var n64 int64
	n64, err = f.raw.ReadAt(f.ctx, p, f.off)
	f.off += n64
	n = int(n64)
	return
}

func (f *fsFile) Write(p []byte) (n int, err error) {
	var n64 int64
	n64, err = f.raw.WriteAt(f.ctx, p, f.off)
	f.off += n64
	n = int(n64)
	return
}

func (f *fsFile) Close() error {
	return f.raw.Close(f.ctx)
}

func (f *fsFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.off = offset
	case io.SeekCurrent:
		f.off += offset
	case io.SeekEnd:
		f.off = f.info.Size()
	}
	return f.off, nil
}

func (f *fsFile) Readdir(count int) ([]FileInfo, error) {
	return nil, types.ErrNoGroup
}

func (f *fsFile) Stat() (FileInfo, error) {
	return f.info, nil
}

func (f *fsFile) Raw() RawFile {
	return f.raw
}

var _ File = &fsFile{}

type fsDIR struct {
	ctx   context.Context
	group int64
	fs    *FileSystem
	info  FileInfo
}

func newDIR(ctx context.Context, entry *types.Entry, fs *FileSystem) *fsDIR {
	return &fsDIR{
		ctx:   ctx,
		group: entry.ID,
		info:  &fInfo{Entry: entry},
		fs:    fs,
	}
}

func (f *fsDIR) Read(p []byte) (n int, err error) {
	return 0, types.ErrIsGroup
}

func (f *fsDIR) Write(p []byte) (n int, err error) {
	return 0, types.ErrIsGroup
}

func (f *fsDIR) Close() error {
	return nil
}

func (f *fsDIR) Seek(offset int64, whence int) (int64, error) {
	return 0, types.ErrIsGroup
}

func (f *fsDIR) Readdir(count int) ([]FileInfo, error) {
	child, err := f.fs.ListChildren(f.ctx, f.group)
	if err != nil {
		return nil, err
	}

	result := make([]FileInfo, 0, len(child))
	for i := range child {
		result = append(result, &fInfo{Entry: child[i]})
	}

	return result, nil
}

func (f *fsDIR) Stat() (FileInfo, error) {
	return f.info, nil
}

func (f *fsDIR) Raw() RawFile {
	return nil
}

var _ File = &fsDIR{}
