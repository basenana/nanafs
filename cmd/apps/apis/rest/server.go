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

package rest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/basenana/nanafs/cmd/apps/apis/rest/docs"
	"github.com/basenana/nanafs/cmd/apps/apis/rest/v1"
	"github.com/basenana/nanafs/config"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/cmd/apps/apis/rest/common"
	"github.com/basenana/nanafs/utils/logger"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const (
	defaultHttpTimeout = time.Minute * 30
)

type Server struct {
	engine    *gin.Engine
	apiConfig config.Config
	logger    *zap.SugaredLogger
	services  *v1.ServicesV1
}

func New(depends *common.Depends, cfg config.Config) (*Server, error) {
	s := &Server{
		engine:    gin.New(),
		apiConfig: cfg,
		logger:    logger.NewLogger("fsapi"),
	}

	s.engine.Use(gin.Recovery())
	s.engine.Use(s.logMiddleware())

	bootCfg := cfg.GetBootstrapConfig()
	s.engine.Use(common.AuthMiddleware(bootCfg.API.JWT))

	docs.SwaggerInfo.BasePath = "/api/v1"
	s.engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	services, err := v1.NewServicesV1(s.engine, depends)
	if err != nil {
		return nil, fmt.Errorf("init services failed: %w", err)
	}
	s.services = services

	v1.RegisterRoutes(s.engine, s.services)

	s.engine.GET("/_ping", s.Ping)
	s.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	pprof.Register(s.engine)

	return s, nil
}

func (s *Server) Run(stopCh chan struct{}) {
	cfg := s.apiConfig.GetBootstrapConfig()
	addr := fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port)
	s.logger.Infof("fsapi server on %s", addr)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  defaultHttpTimeout,
		WriteTimeout: defaultHttpTimeout,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				s.logger.Panicw("rest server down", "err", err)
			}
			s.logger.Infof("fsapi server stopped")
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

func (s *Server) logMiddleware() gin.HandlerFunc {
	return func(gCtx *gin.Context) {
		start := time.Now()
		path := gCtx.Request.URL.Path
		method := gCtx.Request.Method

		gCtx.Next()

		s.logger.Infow("fsapi request",
			"method", method,
			"path", path,
			"query", gCtx.Request.URL.Query().Encode(),
			"status", gCtx.Writer.Status(),
			"duration", time.Since(start).String(),
		)
	}
}
