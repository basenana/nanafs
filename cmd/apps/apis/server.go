package apis

import (
	"fmt"
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
	cfg    config.Api
	logger *zap.SugaredLogger
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodHead:
		s.headHandler(writer, request)
	case http.MethodGet:
		s.getHandler(writer, request)
	case http.MethodPost:
		s.postHandler(writer, request)
	case http.MethodPut:
		s.putHandler(writer, request)
	case http.MethodDelete:
		s.deleteHandler(writer, request)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) Run() error {
	httpServer := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port),
		Handler:        s,
		ReadTimeout:    httpTimeout,
		WriteTimeout:   httpTimeout,
		MaxHeaderBytes: headerSize,
	}
	return httpServer.ListenAndServe()
}

func NewServer(cfg config.Api) (*Server, error) {
	server := &Server{
		cfg:    cfg,
		logger: logger.NewLogger("HttpServer"),
	}
	return server, nil
}
