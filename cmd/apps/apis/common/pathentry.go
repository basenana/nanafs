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

package common

import (
	"context"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"os"
	"path"
	"strings"
)

type PathEntryManager struct {
	ctrl    controller.Controller
	entries *utils.LFUPool
	logger  *zap.SugaredLogger
}

func (m *PathEntryManager) Access(ctx context.Context, entryPath string, callerUid, callGid int64, perm os.FileMode) error {
	entryPath = m.getPath(entryPath)
	entry, err := m.FindEntry(ctx, entryPath)
	if err != nil {
		return err
	}
	return dentry.IsAccess(entry.Metadata().Access, callerUid, callGid, uint32(perm))
}

func (m *PathEntryManager) FindEntry(ctx context.Context, entryPath string) (dentry.Entry, error) {
	entryPath = m.getPath(entryPath)
	pe, err := m.getPathEntry(ctx, entryPath)
	if err != nil {
		return nil, err
	}
	return m.ctrl.GetEntry(ctx, pe.entryID)
}

func (m *PathEntryManager) FindParentEntry(ctx context.Context, entryPath string) (dentry.Entry, error) {
	entryPath = m.getPath(entryPath)
	pe, err := m.getPathEntry(ctx, path.Dir(entryPath))
	if err != nil {
		return nil, err
	}
	return m.ctrl.GetEntry(ctx, pe.entryID)
}

func (m *PathEntryManager) Open(ctx context.Context, entryPath string, attr dentry.Attr) (dentry.Entry, error) {
	entryPath = m.getPath(entryPath)
	pe, err := m.getPathEntry(ctx, entryPath)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}

	if err == types.ErrNotFound {
		if !attr.Create {
			return nil, err
		}
		var en, parent dentry.Entry
		parentDir, base := path.Split(entryPath)
		parent, err = m.FindParentEntry(ctx, parentDir)
		if err != nil {
			return nil, err
		}
		if !parent.IsGroup() {
			return nil, types.ErrNoGroup
		}

		en, err = m.ctrl.CreateEntry(ctx, parent, types.ObjectAttr{
			Name:   base,
			Kind:   types.RawKind,
			Access: parent.Metadata().Access,
		})
		if err != nil {
			return nil, err
		}
		m.logger.Infow("create file entry", "path", entryPath, "entry", en.Metadata().ID)
		return m.ctrl.OpenFile(ctx, en, attr)
	}

	en, err := m.ctrl.GetEntry(ctx, pe.entryID)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}

	if en.IsGroup() {
		return en, nil
	}
	return m.ctrl.OpenFile(ctx, en, attr)
}

func (m *PathEntryManager) CreateAll(ctx context.Context, entryPath string, attr dentry.EntryAttr) (dentry.Entry, error) {
	var (
		en  dentry.Entry
		pEn *pathEntry
		err error
	)
	en, err = m.ctrl.LoadRootEntry(ctx)
	if err != nil {
		return nil, err
	}
	parentPath := m.splitPath(entryPath)
	for _, dirPath := range parentPath {
		if dirPath == "/" {
			continue
		}

		pEn, err = m.getPathEntry(ctx, dirPath)
		if err != nil && err != types.ErrNotFound {
			return nil, err
		}

		if err == types.ErrNotFound {
			pp, base := path.Split(dirPath)
			en, err = m.ctrl.CreateEntry(ctx, en, types.ObjectAttr{Name: base, Kind: types.GroupKind, Access: attr.Access})
			if err != nil {
				return nil, err
			}
			m.logger.Infow("create group entry", "path", dirPath, "entry", en.Metadata().ID)
			m.entries.Put(dirPath, &pathEntry{entryID: en.Metadata().ID, parentPath: pp, baseName: base})
			continue
		}

		en, err = m.ctrl.GetEntry(ctx, pEn.entryID)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (m *PathEntryManager) RemoveAll(ctx context.Context, entryPath string) error {
	paths := m.splitPath(entryPath)
	targetPath := paths[len(paths)-1]

	en, err := m.FindEntry(ctx, targetPath)
	if err != nil {
		if err == types.ErrNotFound {
			return nil
		}
		return err
	}

	parentEn, err := m.FindParentEntry(ctx, targetPath)
	if err != nil {
		return err
	}

	m.logger.Infow("delete entry", "path", targetPath, "entry", en.Metadata().ID)
	if err = m.ctrl.DestroyEntry(ctx, parentEn, en, types.DestroyObjectAttr{}); err != nil {
		return err
	}

	parentPath := paths[:len(paths)-1]
	for i := len(parentPath) - 1; i >= 0; i-- {
		targetPath = parentPath[i]
		en, err = m.FindEntry(ctx, targetPath)
		if err != nil {
			return err
		}
		if !en.IsGroup() {
			return nil
		}
		children, err := en.Group().ListChildren(ctx)
		if err != nil {
			return err
		}
		if len(children) != 0 {
			return nil
		}

		parentEn, err = m.FindParentEntry(ctx, targetPath)
		if err != nil {
			return err
		}

		m.logger.Infow("delete entry", "path", targetPath, "entry", en.Metadata().ID)
		if err = m.ctrl.DestroyEntry(ctx, parentEn, en, types.DestroyObjectAttr{}); err != nil {
			return err
		}
	}
	return nil
}

func (m *PathEntryManager) Rename(ctx context.Context, oldPath, entryPath string) error {
	oldPath = m.getPath(oldPath)
	entryPath = m.getPath(entryPath)
	oldParent, err := m.FindParentEntry(ctx, oldPath)
	if err != nil {
		return err
	}
	newParent, err := m.FindParentEntry(ctx, entryPath)
	if err != nil {
		return err
	}
	target, err := m.FindParentEntry(ctx, oldPath)
	if err != nil {
		return err
	}
	return m.ctrl.ChangeEntryParent(ctx, target, oldParent, newParent, path.Base(entryPath), types.ChangeParentAttr{})
}

func (m *PathEntryManager) getPathEntry(ctx context.Context, entryPath string) (*pathEntry, error) {
	cached := m.entries.Get(entryPath)
	if cached != nil {
		return cached.(*pathEntry), nil
	}

	root, err := m.ctrl.LoadRootEntry(ctx)
	if err != nil {
		return nil, err
	}

	var crt dentry.Entry
	paths := m.splitPath(entryPath)
	for _, p := range paths {
		if p == "/" {
			crt = root
			m.entries.Put(p, &pathEntry{entryID: crt.Metadata().ID, parentPath: p, baseName: p})
			continue
		}
		if crt == nil {
			continue
		}
		parent, base := path.Split(p)
		if base == "." || base == ".." {
			return nil, types.ErrNoAccess
		}

		if !crt.IsGroup() {
			return nil, types.ErrNoGroup
		}

		crt, err = crt.Group().FindEntry(ctx, base)
		if err != nil {
			return nil, err
		}
		m.entries.Put(p, &pathEntry{entryID: crt.Metadata().ID, parentPath: parent, baseName: base})
	}

	cached = m.entries.Get(entryPath)
	if cached != nil {
		return cached.(*pathEntry), nil
	}
	return nil, types.ErrNotFound
}

func (m *PathEntryManager) getPath(entryPath string) string {
	pathParts := strings.Split(entryPath, "/")
	pathEntries := make([]string, 0, len(pathParts))
	for _, p := range pathParts {
		if p == "" {
			continue
		}
		pathEntries = append(pathEntries, p)
	}
	return "/" + strings.Join(pathEntries, "/")
}

func (m *PathEntryManager) splitPath(entryPath string) []string {
	pathParts := strings.Split(entryPath, "/")
	pathEntries := make([]string, 0, len(pathParts))
	for _, p := range pathParts {
		if p == "" {
			continue
		}
		pathEntries = append(pathEntries, p)
	}
	result := make([]string, len(pathEntries)-1)
	for i := 2; i <= len(pathEntries); i++ {
		result[i-2] = "/" + strings.Join(pathEntries[:i], "/")
	}
	result = append([]string{"/"}, result...)
	return result
}

func NewPathEntryManager(controller controller.Controller) *PathEntryManager {
	return &PathEntryManager{
		ctrl:    controller,
		entries: utils.NewLFUPool(1024),
		logger:  logger.NewLogger("PathEntryManager"),
	}
}

type pathEntry struct {
	entryID    int64
	parentPath string
	baseName   string
}
