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
	"github.com/basenana/nanafs/cmd/apps/apis/restfs"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"time"
)

const (
	defaultHttpTimeout = time.Minute * 30
)

type Server struct {
	engine *gin.Engine
	cfg    config.Config
	logger *zap.SugaredLogger
}

func (s *Server) Run(stopCh chan struct{}) {
	addr := fmt.Sprintf("%s:%d", s.cfg.ApiConfig.Host, s.cfg.ApiConfig.Port)
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

func NewApiServer(ctrl controller.Controller, cfg config.Config) (*Server, error) {
	if cfg.ApiConfig.Port == 0 {
		return nil, fmt.Errorf("http port not set")
	}
	if cfg.ApiConfig.Host == "" {
		cfg.ApiConfig.Host = "127.0.0.1"
	}

	s := &Server{
		engine: gin.New(),
		cfg:    cfg,
		logger: logger.NewLogger("api"),
	}
	s.engine.GET("/_ping", s.Ping)
	s.engine.Use()

	if cfg.ApiConfig.Pprof {
		pprof.Register(s.engine)
	}

	if err := restfs.InitRestFs(ctrl, s.engine, cfg); err != nil {
		return nil, fmt.Errorf("init restfs failed: %s", err.Error())
	}
	return s, nil
}
