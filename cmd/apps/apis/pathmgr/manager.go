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

package pathmgr

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
)

type PathManager struct {
	ctrl    controller.Controller
	entries *utils.LFUPool
	logger  *zap.SugaredLogger
}

func (m *PathManager) Controller() controller.Controller {
	return m.ctrl
}

func (m *PathManager) Access(ctx context.Context, entryPath string, callerUid, callGid int64, perm os.FileMode) error {
	var err error
	entry, err := m.FindEntry(ctx, entryPath)
	if err != nil {
		return err
	}
	return dentry.IsAccess(entry.Access, callerUid, callGid, uint32(perm))
}

func (m *PathManager) GetEntry(ctx context.Context, entryID int64) (*types.Entry, error) {
	return m.ctrl.GetEntry(ctx, entryID)
}

func (m *PathManager) FindEntry(ctx context.Context, entryPath string) (*types.Entry, error) {
	var err error
	entryPath, err = m.getPath(ctx, entryPath)
	if err != nil {
		return nil, err
	}
	return m.getPathEntry(ctx, entryPath)
}

func (m *PathManager) ListEntry(ctx context.Context, dirPath string) ([]*types.Entry, error) {
	var err error
	dirPath, err = m.getPath(ctx, dirPath)
	if err != nil {
		return nil, err
	}
	en, err := m.getPathEntry(ctx, dirPath)
	if err != nil {
		return nil, err
	}
	if !en.IsGroup {
		return nil, types.ErrNoGroup
	}
	return m.ctrl.ListEntryChildren(ctx, en.ID, nil, types.Filter{})
}

func (m *PathManager) FindParentEntry(ctx context.Context, entryPath string) (*types.Entry, error) {
	var err error
	entryPath, err = m.getPath(ctx, entryPath)
	if err != nil {
		return nil, err
	}
	return m.getPathEntry(ctx, path.Dir(entryPath))
}

func (m *PathManager) Open(ctx context.Context, enId int64, attr types.OpenAttr) (dentry.File, error) {
	return m.ctrl.OpenFile(ctx, enId, attr)
}

func (m *PathManager) CreateFile(ctx context.Context, parentDir string, attr types.EntryAttr) (*types.Entry, error) {
	var (
		err       error
		result    *types.Entry
		entryPath = path.Join(parentDir, attr.Name)
	)
	if attr.Name == "" {
		return nil, fmt.Errorf("file name is empty")
	}

	entryPath, err = m.getPath(ctx, entryPath)
	if err != nil {
		return nil, err
	}
	result, err = m.getPathEntry(ctx, entryPath)
	if err == nil {
		if result.IsGroup {
			return nil, types.ErrIsGroup
		}
		return result, nil
	} else if err != nil && err != types.ErrNotFound {
		return nil, err
	}

	var en, parent *types.Entry
	parent, err = m.FindEntry(ctx, parentDir)
	if err != nil {
		return nil, err
	}
	if !parent.IsGroup {
		return nil, types.ErrNoGroup
	}

	en, err = m.ctrl.CreateEntry(ctx, parent.ID, attr)
	if err != nil {
		m.logger.Errorw("create file entry failed", "path", entryPath, "err", err)
		return nil, err
	}
	m.logger.Infow("create file entry", "path", entryPath, "entry", en.ID)
	return en, nil
}

func (m *PathManager) CreateAll(ctx context.Context, entryPath string, attr types.EntryAttr) (*types.Entry, error) {
	var (
		en, parent *types.Entry
		err        error
	)
	parent, err = m.ctrl.LoadRootEntry(ctx)
	if err != nil {
		return nil, err
	}
	parentPath := m.splitPath(entryPath)
	for _, dirPath := range parentPath {
		if dirPath == "/" {
			continue
		}

		en, err = m.getPathEntry(ctx, dirPath)
		if err != nil && err != types.ErrNotFound {
			return nil, err
		}

		if err == types.ErrNotFound {
			_, base := path.Split(dirPath)
			en, err = m.ctrl.CreateEntry(ctx, parent.ID, types.EntryAttr{Name: base, Kind: types.GroupKind, Access: attr.Access})
			if err != nil {
				return nil, err
			}
			m.logger.Infow("create group entry", "path", dirPath, "entry", en.ID)
		}
		parent = en
	}

	return en, nil
}

func (m *PathManager) RemoveAll(ctx context.Context, entryPath string, recursion bool) error {
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

	m.logger.Infow("delete entry", "path", targetPath, "entry", en.ID)
	if err = m.ctrl.DestroyEntry(ctx, parentEn.ID, en.ID, types.DestroyObjectAttr{}); err != nil {
		return err
	}

	if !recursion {
		return nil
	}
	parentPath := paths[:len(paths)-1]
	for i := len(parentPath) - 1; i >= 0; i-- {
		targetPath = parentPath[i]
		en, err = m.FindEntry(ctx, targetPath)
		if err != nil {
			return err
		}
		if !en.IsGroup {
			return nil
		}
		children, err := m.ctrl.ListEntryChildren(ctx, en.ID, nil, types.Filter{})
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

		m.logger.Infow("delete entry", "path", targetPath, "entry", en.ID)
		if err = m.ctrl.DestroyEntry(ctx, parentEn.ID, en.ID, types.DestroyObjectAttr{}); err != nil {
			return err
		}
	}
	return nil
}

func (m *PathManager) Rename(ctx context.Context, oldPath, entryPath string) error {
	var err error
	oldPath, err = m.getPath(ctx, oldPath)
	if err != nil {
		return err
	}
	entryPath, err = m.getPath(ctx, entryPath)
	if err != nil {
		return err
	}
	oldParent, err := m.FindParentEntry(ctx, oldPath)
	if err != nil {
		return err
	}
	newParent, err := m.FindParentEntry(ctx, entryPath)
	if err != nil {
		return err
	}
	target, err := m.FindEntry(ctx, oldPath)
	if err != nil {
		return err
	}
	err = m.ctrl.ChangeEntryParent(ctx, target.ID, oldParent.ID, newParent.ID, path.Base(entryPath), types.ChangeParentAttr{})
	if err != nil {
		return err
	}
	return nil
}

func (m *PathManager) getPathEntry(ctx context.Context, entryPath string) (*types.Entry, error) {
	return m.ctrl.GetEntryByURI(ctx, entryPath)
}

func (m *PathManager) getPath(ctx context.Context, entryPath string) (string, error) {
	pathParts := strings.Split(entryPath, "/")
	pathEntries := make([]string, 0, len(pathParts))
	for _, p := range pathParts {
		if p == "" {
			continue
		}
		if p == "." || p == ".." {
			return "", types.ErrNoPerm
		}
		pathEntries = append(pathEntries, p)
	}
	entryPath = "/" + strings.Join(pathEntries, "/")
	return entryPath, nil
}

func (m *PathManager) splitPath(entryPath string) []string {
	if entryPath == "/" {
		return []string{entryPath}
	}
	pathParts := strings.Split(entryPath, "/")
	pathEntries := make([]string, 0, len(pathParts))
	for _, p := range pathParts {
		if p == "" {
			continue
		}
		pathEntries = append(pathEntries, p)
	}
	result := make([]string, 0, len(pathEntries))
	for i := 0; i <= len(pathEntries); i++ {
		result = append(result, "/"+strings.Join(pathEntries[:i], "/"))
	}
	return result
}

func New(controller controller.Controller) (*PathManager, error) {
	_, err := controller.LoadRootEntry(context.Background())
	if err != nil {
		return nil, err
	}
	return &PathManager{
		ctrl:   controller,
		logger: logger.NewLogger("PathManager"),
	}, nil
}
