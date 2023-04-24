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
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/common"
	"github.com/basenana/nanafs/cmd/apps/apis/pathmgr"
	"github.com/basenana/nanafs/config"
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

var log *zap.SugaredLogger

type Webdav struct {
	cfg     config.Webdav
	handler http.Handler
	logger  *zap.SugaredLogger
}

func (w *Webdav) Run(stopCh chan struct{}) {
	addr := fmt.Sprintf("%s:%d", w.cfg.Host, w.cfg.Port)
	w.logger.Infof("webdav server on %s", addr)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      common.BasicAuthHandler(w.handler, w.cfg.OverwriteUsers),
		ReadTimeout:  time.Hour,
		WriteTimeout: time.Hour,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
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
	mgr    *pathmgr.PathManager
	cfg    config.Webdav
	logger *zap.SugaredLogger
}

func (o FsOperator) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	defer trace.StartRegion(ctx, "apis.webdav.Mkdir").End()
	_, err := o.mgr.CreateAll(ctx, name, mode2EntryAttr(perm))
	return error2FsError(err)
}

func (o FsOperator) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	defer trace.StartRegion(ctx, "apis.webdav.OpenFile").End()
	userInfo := common.GetUserInfo(ctx)
	if userInfo == nil {
		return nil, error2FsError(types.ErrNoAccess)
	}

	openAttr := flag2EntryOpenAttr(flag)
	err := o.mgr.Access(ctx, name, userInfo.UID, userInfo.GID, perm)
	if err == nil {
		en, err := o.mgr.FindEntry(ctx, name)
		if err != nil {
			return nil, error2FsError(err)
		}
		f, err := openFile(en, o.mgr, openAttr)
		if err != nil {
			return nil, error2FsError(err)
		}
		return f, err
	} else if err != nil && err != types.ErrNotFound {
		return nil, error2FsError(err)
	}

	if err == types.ErrNotFound {
		if !openAttr.Create {
			return nil, error2FsError(err)
		}
		_, err = o.mgr.CreateAll(ctx, path.Dir(name), mode2EntryAttr(perm))
		if err != nil {
			return nil, error2FsError(err)
		}
	}
	parentDir, filename := path.Split(name)
	en, err := o.mgr.CreateFile(ctx, parentDir, types.ObjectAttr{
		Name:   filename,
		Kind:   types.RawKind,
		Access: mode2EntryAttr(perm).Access,
	})
	if err != nil {
		return nil, error2FsError(err)
	}
	f, err := openFile(en, o.mgr, openAttr)
	if err != nil {
		return nil, error2FsError(err)
	}
	return f, nil
}

func (o FsOperator) RemoveAll(ctx context.Context, name string) error {
	defer trace.StartRegion(ctx, "apis.webdav.RemoveAll").End()
	return error2FsError(o.mgr.RemoveAll(ctx, name, false))
}

func (o FsOperator) Rename(ctx context.Context, oldName, newName string) error {
	defer trace.StartRegion(ctx, "apis.webdav.Rename").End()
	return error2FsError(o.mgr.Rename(ctx, oldName, newName))
}

func (o FsOperator) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	defer trace.StartRegion(ctx, "apis.webdav.Stat").End()
	en, err := o.mgr.FindEntry(ctx, name)
	if err != nil {
		return nil, error2FsError(err)
	}
	return Stat(en.Metadata()), nil
}

func NewWebdavServer(mgr *pathmgr.PathManager, cfg config.Webdav) (*Webdav, error) {
	if cfg.Port == 0 {
		return nil, fmt.Errorf("http port not set")
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}

	log = logger.NewLogger("webdav")
	w := FsOperator{mgr: mgr, cfg: cfg, logger: log}
	handler := &webdav.Handler{
		FileSystem: w,
		LockSystem: webdav.NewMemLS(), // TODO:need flock
		Logger:     initLogger(log).handle,
	}
	return &Webdav{cfg: cfg, handler: handler, logger: log}, nil
}
