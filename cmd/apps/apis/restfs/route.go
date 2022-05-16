package restfs

import (
	"github.com/basenana/nanafs/cmd/apps/apis/restfs/v1"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/gin-gonic/gin"
)

const (
	restFsPath = "/v1/fs/*path"
)

type Handler interface {
	Get(gCtx *gin.Context)
	Post(gCtx *gin.Context)
	Put(gCtx *gin.Context)
	Delete(gCtx *gin.Context)
}

func InitRestFs(ctrl controller.Controller, engine *gin.Engine, cfg config.Config) error {
	if !cfg.ApiConfig.Enable {
		return nil
	}

	v1Handler := v1.NewRestFs(ctrl, cfg)

	engine.GET(restFsPath, v1Handler.Get)
	engine.POST(restFsPath, v1Handler.Post)
	engine.PUT(restFsPath, v1Handler.Put)
	engine.DELETE(restFsPath, v1Handler.Delete)
	return nil
}
