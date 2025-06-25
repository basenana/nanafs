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
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/fsapi/common"
	"github.com/basenana/nanafs/pkg/core"
	"net/http"
	"time"

	"github.com/basenana/nanafs/cmd/apps/apis/fsapi"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/cmd/apps/apis/webdav"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	defaultHttpTimeout = time.Minute * 30
)

type Server struct {
	engine    *gin.Engine
	apiConfig config.Config
	logger    *zap.SugaredLogger
}

func (s *Server) Run(stopCh chan struct{}) {
	apiHost, err := s.apiConfig.GetSystemConfig(context.TODO(), config.AdminApiConfigGroup, "host").String()
	if err != nil {
		s.logger.Errorw("query admin api host config failed, skip", "err", err)
		return
	}
	apiPort, err := s.apiConfig.GetSystemConfig(context.TODO(), config.AdminApiConfigGroup, "port").Int()
	if err != nil {
		s.logger.Errorw("query admin api port config failed, skip", "err", err)
		return
	}

	addr := fmt.Sprintf("%s:%d", apiHost, apiPort)
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
				s.logger.Panicw("api server down", "err", err)
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

func NewHttpApiServer(apiConfig config.Config) (*Server, error) {
	s := &Server{
		engine:    gin.New(),
		apiConfig: apiConfig,
		logger:    logger.NewLogger("api"),
	}

	s.engine.GET("/_ping", s.Ping)

	enableMetric, err := apiConfig.GetSystemConfig(context.TODO(), config.AdminApiConfigGroup, "enable_metric").Bool()
	if err != nil {
		s.logger.Warnw("query enable metric config failed, skip", "err", err)
	}
	enablePprof, err := apiConfig.GetSystemConfig(context.TODO(), config.AdminApiConfigGroup, "enable_pprof").Bool()
	if err != nil {
		s.logger.Warnw("query enable pprof config failed, skip", "err", err)
	}

	if enableMetric {
		s.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	if enablePprof {
		pprof.Register(s.engine)
	}

	return s, nil
}

func RunFSAPI(depends *common.Depends, cfg config.Config, stopCh chan struct{}) error {
	s, err := fsapi.New(depends, cfg)
	if err != nil {
		return fmt.Errorf("init fsapi server failed: %w", err)
	}
	go s.Run(stopCh)
	return nil
}

func RunWebdav(fs *core.FileSystem, cfg config.Webdav, stopCh chan struct{}) error {
	if !cfg.Enable {
		return nil
	}
	w, err := webdav.NewWebdavServer(fs, cfg)
	if err != nil {
		return fmt.Errorf("init webdav server failed: %w", err)
	}
	go w.Run(stopCh)
	return nil
}
