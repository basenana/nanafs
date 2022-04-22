package apis

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/restfs"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"net/http"
	"time"
)

const (
	httpTimeout = time.Minute
	headerSize  = 1 << 20
)

type Server struct {
	httpHandler *http.ServeMux
	cfg         config.Api
	logger      *zap.SugaredLogger
}

func (s *Server) Run(stopCh chan struct{}) {
	httpServer := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port),
		Handler:        s.httpHandler,
		ReadTimeout:    httpTimeout,
		WriteTimeout:   httpTimeout,
		MaxHeaderBytes: headerSize,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				s.logger.Panicw("http server failed", "err", err.Error())
			}
			s.logger.Infow("http server shutdown")
		}
	}()

	go func() {
		<-stopCh
		ctx, fn := context.WithTimeout(context.Background(), time.Second)
		defer fn()
		_ = httpServer.Shutdown(ctx)
	}()
}

func NewServer(cfg config.Api) (*Server, error) {
	if cfg.Port == 0 {
		return nil, fmt.Errorf("http port not set")
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}

	log := logger.NewLogger("http")
	log.Infof("init http server on %s:%d", cfg.Host, cfg.Port)
	var (
		route = map[string]http.HandlerFunc{}
		err   error
	)

	initMetric(cfg, route)
	if err = restfs.InitRestFs(cfg, route); err != nil {
		return nil, err
	}

	httpHandler := http.NewServeMux()
	for path, handler := range route {
		log.Infof("register api %s", path)
		httpHandler.HandleFunc(path, handler)
	}

	s := &Server{
		httpHandler: httpHandler,
		cfg:         cfg,
		logger:      log,
	}
	return s, nil
}
