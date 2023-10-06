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
	"net/http"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

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
	apiConfig config.Api
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

func NewApiServer(mgr *pathmgr.PathManager, cfg config.Config) (*Server, error) {
	apiConfig := cfg.Api
	if apiConfig.Enable && apiConfig.Port == 0 {
		return nil, fmt.Errorf("http port not set")
	}
	if apiConfig.Enable && apiConfig.Host == "" {
		cfg.Api.Host = "127.0.0.1"
	}

	s := &Server{
		engine:    gin.New(),
		apiConfig: apiConfig,
		logger:    logger.NewLogger("api"),
	}

	s.engine.GET("/_ping", s.Ping)
	s.engine.POST("/friday/query", apifriday.Query)

	if apiConfig.Metrics {
		s.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	if apiConfig.Pprof {
		pprof.Register(s.engine)
	}

	return s, nil
}

func NewWebdavServer(mgr *pathmgr.PathManager, cfg config.Config) (*webdav.Webdav, error) {
	if cfg.Webdav == nil || !cfg.Webdav.Enable {
		return nil, fmt.Errorf("webdav not enable")
	}
	return webdav.NewWebdavServer(mgr, *cfg.Webdav)
}
