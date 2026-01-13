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
	"errors"
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"golang.org/x/net/webdav"
	"net/http"
	"os"
	"path"
	"runtime/trace"
	"time"
)

var (
	log *zap.SugaredLogger
)

type Webdav struct {
	handler http.Handler
	cfg     config.Webdav
	logger  *zap.SugaredLogger
}

func (w *Webdav) Run(stopCh chan struct{}) {
	addr := fmt.Sprintf("%s:%d", w.cfg.Host, w.cfg.Port)
	w.logger.Infof("webdav server on %s", addr)

	handler := apitool.MetricMiddleware("webdav", w.handler)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  time.Hour,
		WriteTimeout: time.Hour,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			if !errors.Is(http.ErrServerClosed, err) {
				w.logger.Errorw("webdav server down", "err", err)
			}
			w.logger.Infof("webdav server stopped")
		}
	}()

	<-stopCh
	shutdownCtx, canF := context.WithTimeout(context.TODO(), time.Second)
	defer canF()
	_ = httpServer.Shutdown(shutdownCtx)
}

type FsOperator struct {
	fs     *core.FileSystem
	root   *types.Entry
	cfg    config.Webdav
	logger *zap.SugaredLogger
}

func (o FsOperator) Mkdir(ctx context.Context, p string, perm os.FileMode) error {
	defer trace.StartRegion(ctx, "apis.webdav.Mkdir").End()

	var (
		attr   = mode2EntryAttr(perm)
		crtURI = "/"
	)

	for _, ename := range splitPath(p) {
		parentURI := crtURI
		childURI := path.Join(crtURI, ename)

		_, _, err := o.fs.GetEntryByPath(ctx, childURI)
		if err != nil && !errors.Is(err, types.ErrNotFound) {
			return error2FsError(err)
		}

		if err == types.ErrNotFound {
			// create
			attr.Name = ename
			_, err = o.fs.CreateEntry(ctx, parentURI, attr)
			if err != nil {
				return error2FsError(err)
			}
		}
		crtURI = childURI
	}

	return nil
}

func (o FsOperator) OpenFile(ctx context.Context, entryPath string, flag int, perm os.FileMode) (webdav.File, error) {
	defer trace.StartRegion(ctx, "apis.webdav.OpenFile").End()
	userInfo := getUserInfo(ctx)
	if userInfo == nil {
		return nil, error2FsError(types.ErrNoAccess)
	}

	var err error
	parent, entry, err := o.fs.GetEntryByPath(ctx, entryPath)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, error2FsError(err)
	}

	openAttr := flag2EntryOpenAttr(flag)
	if errors.Is(err, types.ErrNotFound) {
		if !openAttr.Create {
			return nil, error2FsError(err)
		}

		parentDir, filename := path.Split(entryPath)
		_, parent, err = o.fs.GetEntryByPath(ctx, parentDir)
		if err != nil {
			return nil, error2FsError(err)
		}

		if err = core.HasAllPermissions(parent.Access, userInfo.UID, userInfo.GID,
			types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite); err != nil {
			return nil, err
		}

		attr := mode2EntryAttr(perm)
		attr.Name = filename
		entry, err = o.fs.CreateEntry(ctx, parentDir, attr)
		if err != nil {
			return nil, error2FsError(err)
		}
	}

	if err = core.IsAccess(entry.Access, userInfo.UID, userInfo.GID, uint32(perm)); err != nil {
		return nil, err
	}

	f, err := openFile(ctx, entry, o.fs, openAttr)
	if err != nil {
		return nil, error2FsError(err)
	}
	return f, err
}

func (o FsOperator) RemoveAll(ctx context.Context, entryPath string) error {
	defer trace.StartRegion(ctx, "apis.webdav.RemoveAll").End()

	parent, en, err := o.fs.GetEntryByPath(ctx, entryPath)
	if err != nil {
		return error2FsError(err)
	}

	if parent.ID == en.ID { // root entry
		return error2FsError(types.ErrNoPerm)
	}

	if !en.IsGroup {
		err := o.fs.UnlinkEntry(ctx, entryPath, types.DestroyEntryAttr{})
		if err != nil {
			return error2FsError(err)
		}
		return nil
	}

	err = o.fs.RmGroup(ctx, entryPath, types.DestroyEntryAttr{Recursion: true})
	if err != nil {
		return error2FsError(types.ErrNotEmpty)
	}

	return nil
}

func (o FsOperator) Rename(ctx context.Context, oldPath, newPath string) error {
	defer trace.StartRegion(ctx, "apis.webdav.Rename").End()
	oldName := path.Base(oldPath)
	oldParent, target, err := o.fs.GetEntryByPath(ctx, oldPath)
	if err != nil {
		return error2FsError(err)
	}

	if oldParent.ID == target.ID { // root entry
		return error2FsError(types.ErrNoPerm)
	}

	newParent := path.Dir(newPath)
	newName := path.Base(newPath)
	_, parent, err := o.fs.GetEntryByPath(ctx, newParent)
	if err != nil {
		return error2FsError(err)
	}

	err = o.fs.Rename(ctx, target.ID, oldParent.ID, parent.ID, oldName, newName, types.ChangeParentAttr{})
	if err != nil {
		return error2FsError(err)
	}

	return nil
}

func (o FsOperator) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	defer trace.StartRegion(ctx, "apis.webdav.Stat").End()

	_, en, err := o.fs.GetEntryByPath(ctx, path)
	if err != nil {
		return nil, error2FsError(err)
	}
	return Stat(en), nil
}

func NewWebdavServer(fs *core.FileSystem, cfg config.Webdav) (*Webdav, error) {
	log = logger.NewLogger("webdav")
	w := FsOperator{fs: fs, cfg: cfg, logger: log}

	root, err := fs.Root(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load namespace root failed")
	}
	w.root = root

	handler := &webdav.Handler{
		FileSystem: w,
		LockSystem: webdav.NewMemLS(), // TODO:need flock
		Logger:     logger.InitWebdavLogger().Handle,
	}
	return &Webdav{
		handler: basicAuthHandler(handler, fs.Namespace(), cfg),
		cfg:     cfg, logger: log,
	}, nil
}
