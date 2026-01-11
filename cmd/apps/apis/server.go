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

	"github.com/basenana/nanafs/cmd/apps/apis/rest"
	restcommon "github.com/basenana/nanafs/cmd/apps/apis/rest/common"
	"github.com/basenana/nanafs/cmd/apps/apis/webdav"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/utils/logger"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
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
	addr := fmt.Sprintf("127.0.0.1:8080")
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
	s.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	pprof.Register(s.engine)

	return s, nil
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

func RunFSAPI(depends *restcommon.Depends, cfg config.Config, stopCh chan struct{}) error {
	restDepends := &restcommon.Depends{
		Meta:       depends.Meta,
		Workflow:   depends.Workflow,
		Dispatcher: depends.Dispatcher,
		Notify:     depends.Notify,
		Config:     depends.Config,
		Core:       depends.Core,
	}
	s, err := rest.New(restDepends, cfg)
	if err != nil {
		return fmt.Errorf("init fsapi server failed: %w", err)
	}
	go s.Run(stopCh)
	return nil
}
