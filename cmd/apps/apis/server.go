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

package apis

import (
	"context"
	"errors"
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/fsapi"
	"net/http"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	apifeed "github.com/basenana/nanafs/cmd/apps/apis/document"
	apifriday "github.com/basenana/nanafs/cmd/apps/apis/friday"
	"github.com/basenana/nanafs/cmd/apps/apis/pathmgr"
	"github.com/basenana/nanafs/cmd/apps/apis/webdav"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	defaultHttpTimeout = time.Minute * 30
)

type Server struct {
	engine    *gin.Engine
	apiConfig config.HttpApi
	logger    *zap.SugaredLogger
}

func (s *Server) Run(stopCh chan struct{}) {
	addr := fmt.Sprintf("%s:%d", s.apiConfig.Host, s.apiConfig.Port)
	s.logger.Infof("http server on %s", addr)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  defaultHttpTimeout,
		WriteTimeout: defaultHttpTimeout,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				s.logger.Panicw("api server down", "err", err.Error())
			}
			s.logger.Infof("api server stopped")
		}
	}()

	<-stopCh
	shutdownCtx, canF := context.WithTimeout(context.TODO(), time.Second)
	defer canF()
	_ = httpServer.Shutdown(shutdownCtx)
}

func (s *Server) Ping(gCtx *gin.Context) {
	gCtx.JSON(200, map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func NewPathEntryManager(ctrl controller.Controller) (*pathmgr.PathManager, error) {
	return pathmgr.New(ctrl)
}

func NewHttpApiServer(ctrl controller.Controller, mgr *pathmgr.PathManager, apiConfig config.HttpApi) (*Server, error) {
	docAPIServer := apifeed.NewDocumentAPIServer(ctrl)
	s := &Server{
		engine:    gin.New(),
		apiConfig: apiConfig,
		logger:    logger.NewLogger("api"),
	}

	s.engine.GET("/_ping", s.Ping)
	s.engine.POST("/friday/question", apifriday.Question)
	s.engine.GET("/document/summary", docAPIServer.Summary)
	s.engine.GET("/document/query", docAPIServer.Query)
	s.engine.GET("/feed/:feedId/atom.xml", docAPIServer.Atom)

	if apiConfig.Metrics {
		s.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	if apiConfig.Pprof {
		pprof.Register(s.engine)
	}

	return s, nil
}

func Setup(ctrl controller.Controller, pathEntryMgr *pathmgr.PathManager, cfg config.Loader, stopCh chan struct{}) error {
	var ctx = context.Background()

	fsAPIEnable, err := cfg.GetSystemConfig(ctx, config.FsAPIConfigGroup, "enable").Bool()
	if err != nil && !errors.Is(err, config.ErrNotConfigured) {
		return fmt.Errorf("get fs api enable config failed: %w", err)
	}
	if fsAPIEnable {
		s, err := fsapi.New(ctrl, pathEntryMgr, cfg)
		if err != nil {
			return fmt.Errorf("init fsapi server failed: %w", err)
		}
		go s.Run(stopCh)
	}

	adminAPIEnable, err := cfg.GetSystemConfig(ctx, config.AdminApiConfigGroup, "enable").Bool()
	if err != nil && !errors.Is(err, config.ErrNotConfigured) {
		return fmt.Errorf("get admin api enable config failed: %w", err)
	}
	if adminAPIEnable {
		s, err := NewHttpApiServer(ctrl, pathEntryMgr, cfg)
		if err != nil {
			return fmt.Errorf("init http server failed: %w", err)
		}
		go s.Run(stopCh)
	}

	webdavEnable, err := cfg.GetSystemConfig(ctx, config.WebdavConfigGroup, "enable").Bool()
	if err != nil && !errors.Is(err, config.ErrNotConfigured) {
		return fmt.Errorf("get webdav api enable config failed: %w", err)
	}
	if webdavEnable {
		w, err := webdav.NewWebdavServer(pathEntryMgr, cfg)
		if err != nil {
			return fmt.Errorf("init webdav server failed: %w", err)
		}
		go w.Run(stopCh)
	}

	return nil
}
