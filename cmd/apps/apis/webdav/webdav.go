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

package webdav

import (
	"context"
	"github.com/basenana/nanafs/cmd/apps/apis/common"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"golang.org/x/net/webdav"
	"net/http"
	"os"
	"path"
	"runtime/trace"
)

type Webdav struct {
	mgr    *common.PathEntryManager
	cfg    config.WebdavConfig
	logger *zap.SugaredLogger
}

func (w Webdav) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	defer trace.StartRegion(ctx, "apis.webdav.Mkdir").End()
	_, err := w.mgr.CreateAll(ctx, name, mode2EntryAttr(perm))
	return err
}

func (w Webdav) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	defer trace.StartRegion(ctx, "apis.webdav.OpenFile").End()
	err := w.mgr.Access(ctx, name, w.cfg.UID, w.cfg.GID, perm)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}
	openAttr := flag2EntryOpenAttr(flag)
	if err == types.ErrNotFound {
		if !openAttr.Create {
			return nil, err
		}
		_, err = w.mgr.CreateAll(ctx, path.Dir(name), mode2EntryAttr(perm))
		if err != nil {
			return nil, err
		}
	}
	en, err := w.mgr.Open(ctx, name, openAttr)
	if err != nil {
		return nil, err
	}
	return openFile(en)
}

func (w Webdav) RemoveAll(ctx context.Context, name string) error {
	defer trace.StartRegion(ctx, "apis.webdav.RemoveAll").End()
	return w.mgr.RemoveAll(ctx, name)
}

func (w Webdav) Rename(ctx context.Context, oldName, newName string) error {
	defer trace.StartRegion(ctx, "apis.webdav.Rename").End()
	return w.mgr.Rename(ctx, oldName, newName)
}

func (w Webdav) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	defer trace.StartRegion(ctx, "apis.webdav.Stat").End()
	en, err := w.mgr.Open(ctx, name, dentry.Attr{})
	if err != nil {
		return nil, err
	}
	f, err := openFile(en)
	if err != nil {
		return nil, err
	}
	return f.Stat()
}

func NewHandler(cfg config.WebdavConfig, mgr *common.PathEntryManager) http.Handler {
	w := Webdav{
		mgr:    mgr,
		cfg:    cfg,
		logger: logger.NewLogger("webdav"),
	}

	handler := &webdav.Handler{
		FileSystem: w,
		LockSystem: webdav.NewMemLS(), // TODO:need flock
		Logger:     initLogger().handle,
	}
	return handler
}
