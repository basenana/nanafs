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
	"github.com/basenana/nanafs/pkg/controller"
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
	log       *zap.SugaredLogger
	namespace = types.DefaultNamespace
)

type Webdav struct {
	handler http.Handler
	cfg     config.Loader
	logger  *zap.SugaredLogger
}

func (w *Webdav) Run(stopCh chan struct{}) {
	webdavHost, err := w.cfg.GetSystemConfig(context.TODO(), config.WebdavConfigGroup, "host").String()
	if err != nil {
		w.logger.Errorw("query webdav host config failed, skip", "err", err)
		return
	}
	webdavPort, err := w.cfg.GetSystemConfig(context.TODO(), config.WebdavConfigGroup, "port").Int()
	if err != nil {
		w.logger.Errorw("query webdav port config failed, skip", "err", err)
		return
	}

	addr := fmt.Sprintf("%s:%d", webdavHost, webdavPort)
	w.logger.Infof("webdav server on %s", addr)

	handler := apitool.MetricMiddleware("webdav", w.handler)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  time.Hour,
		WriteTimeout: time.Hour,
	}

	go func() {
		if err = httpServer.ListenAndServe(); err != nil {
			if !errors.Is(http.ErrServerClosed, err) {
				w.logger.Panicw("webdav server down", "err", err.Error())
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
	fs        *core.FS
	namespace string
	root      *core.Entry
	cfg       config.Loader
	logger    *zap.SugaredLogger
}

func (o FsOperator) Mkdir(ctx context.Context, path string, perm os.FileMode) error {
	defer trace.StartRegion(ctx, "apis.webdav.Mkdir").End()

	var (
		attr  = mode2EntryAttr(perm)
		crtID = o.root.ID
	)

	for _, ename := range splitPath(path) {
		en, err := o.fs.LookUpEntry(ctx, namespace, crtID, ename)
		if err != nil && !errors.Is(err, types.ErrNotFound) {
			return error2FsError(err)
		}
		en.Close()

		if en == nil {
			// create
			attr.Name = ename
			en, err = o.fs.CreateEntry(ctx, namespace, crtID, attr)
			if err != nil {
				return error2FsError(err)
			}
		}
		crtID = en.ID
		en.Close()
	}

	return nil
}

func (o FsOperator) OpenFile(ctx context.Context, entryPath string, flag int, perm os.FileMode) (webdav.File, error) {
	defer trace.StartRegion(ctx, "apis.webdav.OpenFile").End()
	userInfo := apitool.GetUserInfo(ctx)
	if userInfo == nil {
		return nil, error2FsError(types.ErrNoAccess)
	}

	var err error
	parent, entry, err := o.fs.GetEntryByPath(ctx, namespace, entryPath)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, error2FsError(err)
	}

	openAttr := flag2EntryOpenAttr(flag)
	if errors.Is(err, types.ErrNotFound) {
		if !openAttr.Create {
			return nil, error2FsError(err)
		}

		var pparent *core.Entry
		parentDir, filename := path.Split(entryPath)
		pparent, parent, err = o.fs.GetEntryByPath(ctx, namespace, parentDir)
		if err != nil {
			return nil, error2FsError(err)
		}
		defer pparent.Close()

		if err = core.IsHasPermissions(parent.Access, userInfo.UID, userInfo.GID,
			types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite); err != nil {
			return nil, err
		}

		attr := mode2EntryAttr(perm)
		attr.Name = filename
		entry, err = o.fs.CreateEntry(ctx, namespace, parent.ID, attr)
		if err != nil {
			return nil, error2FsError(err)
		}
	}

	defer parent.Close()
	if err = core.IsAccess(entry.Access, userInfo.UID, userInfo.GID, uint32(perm)); err != nil {
		return nil, err
	}

	f, err := openFile(ctx, entry, o.fs, openAttr)
	if err != nil {
		return nil, error2FsError(err)
	}
	return f, err
}

func (o FsOperator) RemoveAll(ctx context.Context, path string) error {
	defer trace.StartRegion(ctx, "apis.webdav.RemoveAll").End()

	parent, en, err := o.fs.GetEntryByPath(ctx, o.namespace, path)
	if err != nil {
		return error2FsError(err)
	}
	defer parent.Close()
	defer en.Close()

	if !en.IsGroup {
		err := o.fs.UnlinkEntry(ctx, namespace, parent.ID, en.Name, types.DestroyEntryAttr{})
		if err != nil {
			return error2FsError(err)
		}
		return nil
	}

	err = o.fs.RemoveGroup(ctx, namespace, parent.ID, en.Name, types.DestroyEntryAttr{Recursion: true})
	if err != nil {
		return error2FsError(types.ErrNotEmpty)
	}

	return nil
}

func (o FsOperator) Rename(ctx context.Context, oldPath, newPath string) error {
	defer trace.StartRegion(ctx, "apis.webdav.Rename").End()
	oldParent, target, err := o.fs.GetEntryByPath(ctx, o.namespace, oldPath)
	if err != nil {
		return error2FsError(err)
	}
	defer oldParent.Close()
	defer target.Close()

	newParent := path.Dir(newPath)
	newName := path.Base(newPath)
	pparent, parent, err := o.fs.GetEntryByPath(ctx, o.namespace, newParent)
	if err != nil {
		return error2FsError(err)
	}
	defer pparent.Close()
	defer parent.Close()

	err = o.fs.Rename(ctx, o.namespace, target.ID, oldParent.ID, parent.ID, newName, types.ChangeParentAttr{})
	if err != nil {
		return error2FsError(err)
	}

	return nil
}

func (o FsOperator) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	defer trace.StartRegion(ctx, "apis.webdav.Stat").End()

	parent, en, err := o.fs.GetEntryByPath(ctx, o.namespace, path)
	if err != nil {
		return nil, error2FsError(err)
	}
	defer parent.Close()
	defer en.Close()
	return Stat(en), nil
}

func (o FsOperator) closeEntries(entries []*core.Entry) {
	for _, en := range entries {
		if en.ID == o.root.ID {
			continue
		}
		en.Close()
	}
}

func NewWebdavServer(fs *core.FS, ctrl controller.Controller, cfg config.Loader) (*Webdav, error) {
	log = logger.NewLogger("webdav")
	w := FsOperator{fs: fs, cfg: cfg, logger: log}

	root, err := fs.GetNamespaceRoot(context.Background(), namespace)
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
		handler: apitool.BasicAuthHandler(handler, ctrl),
		cfg:     cfg, logger: log,
	}, nil
}
