package controller

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

type Controller interface {
	EntryController
	FileController
	FsController
}

type controller struct {
	meta      storage.MetaStore
	storage   storage.Storage
	cfgLoader config.Loader
	logger    *zap.SugaredLogger
}

var _ Controller = &controller{}

func New(loader config.Loader, meta storage.MetaStore, storage storage.Storage) Controller {
	return &controller{
		meta:      meta,
		storage:   storage,
		cfgLoader: loader,
		logger:    logger.NewLogger("controller"),
	}
}
