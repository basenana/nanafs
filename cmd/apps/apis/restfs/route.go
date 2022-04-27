package restfs

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/gin-gonic/gin"
)

const (
	restFsPath = "/fs/*path"
)

func InitRestFs(ctrl controller.Controller, engine *gin.Engine, cfg config.Api) error {
	if !cfg.Enable {
		return nil
	}
	s := &RestFS{
		cfg:    cfg,
		ctrl:   ctrl,
		logger: logger.NewLogger("HttpServer"),
	}

	engine.GET(restFsPath, s.Get)
	engine.POST(restFsPath, s.Post)
	engine.PUT(restFsPath, s.Put)
	engine.DELETE(restFsPath, s.Delete)
	return nil
}
