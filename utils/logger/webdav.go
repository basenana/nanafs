package logger

import (
	"go.uber.org/zap"
	"io/fs"
	"net/http"
)

type WebdavLogger struct {
	logger *zap.SugaredLogger
}

func (l *WebdavLogger) Handle(req *http.Request, err error) {
	if err != nil && err != fs.ErrNotExist {
		l.logger.Errorw(req.URL.Path, "method", req.Method, "err", err)
		return
	}
	l.logger.Infow(req.URL.Path, "method", req.Method)
}

func InitWebdavLogger() *WebdavLogger {
	return &WebdavLogger{logger: root.Named("webdav")}
}
