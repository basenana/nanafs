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
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"go.uber.org/zap"
	"math"
	"os"
	"runtime/trace"
	"time"
)

type FS struct {
	core      Core
	store     metastore.Meta
	dentry    DCache
	namespace string
	logger    *zap.SugaredLogger
}

func (f *FS) FsInfo(ctx context.Context) Info {
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

func (f *FS) Access(ctx context.Context, namespace, entryPath string, callerUid, callGid int64, perm os.FileMode) error {
}

func (f *FS) GetNamespaceRoot(ctx context.Context, namespace string) (*Entry, error) {
	return nil, nil
}

func (f *FS) GetEntry(ctx context.Context, namespace string, id int64) (*Entry, error) {
	return nil, nil
}

func (f *FS) GetEntryByPath(ctx context.Context, namespace string, path string) (*Entry, *Entry, error) {
	return nil, nil, nil
}

func (f *FS) LookUpEntry(ctx context.Context, namespace string, parent int64, name string) (*Entry, error) {
	return nil, nil
}

func (f *FS) CreateEntry(ctx context.Context, namespace string, parent int64, newEn types.EntryAttr) (*Entry, error) {
	return nil, nil
}

func (f *FS) UpdateEntry(ctx context.Context, namespace string, id int64, update types.UpdateEntry) (*Entry, error) {
	return nil, nil
}

// MARK: tree

func (f *FS) LinkEntry(ctx context.Context, namespace string, srcEntryId, dstParentId int64, newEn types.EntryAttr) (*Entry, error) {
	return nil, nil
}

func (f *FS) UnlinkEntry(ctx context.Context, namespace string, parent int64, child string, attr types.DestroyEntryAttr) error {
	return nil
}

func (f *FS) RemoveGroup(ctx context.Context, namespace string, parent int64, child string, attr types.DestroyEntryAttr) error {
	return nil
}

func (f *FS) Rename(ctx context.Context, namespace string, targetId, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error {
	return nil
}

// MARK: xattr

func (f *FS) GetXAttr(ctx context.Context, namespace string, id int64, fKey string) ([]byte, error) {
	return nil, nil
}

func (f *FS) SetXAttr(ctx context.Context, namespace string, id int64, fKey string, fVal []byte) error {
	defer trace.StartRegion(ctx, "fs.SetXAttr").End()
	f.logger.Debugw("set entry extend filed", "entry", id, "key", fKey)

	if err := f.store.AddEntryProperty(ctx, id, fKey, types.PropertyItem{Value: utils.EncodeBase64(fVal), Encoded: true}); err != nil {
		return err
	}
	return nil
}

func (f *FS) RemoveXAttr(ctx context.Context, namespace string, id int64, fKey string) error {
	return nil
}

// MARK: file

func (f *FS) Open(ctx context.Context, namespace string, id int64, attr types.OpenAttr) (File, error) {
	return nil, nil
}

func (f *FS) OpenDir(ctx context.Context, namespace string, id int64) (File, error) {
	return nil, nil
}
