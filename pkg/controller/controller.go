package controller

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/group"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

type Controller interface {
	ObjectController
	FileController
	FsController
}

type controller struct {
	meta      storage.MetaStore
	storage   storage.Storage
	cfg       config.Config
	cfgLoader config.Loader

	group    *group.Manager
	registry *dentry.SchemaRegistry

	logger *zap.SugaredLogger
}

var _ Controller = &controller{}

func New(loader config.Loader, meta storage.MetaStore, storage storage.Storage) Controller {
	cfg, _ := loader.GetConfig()

	ctl := &controller{
		meta:      meta,
		storage:   storage,
		cfg:       cfg,
		cfgLoader: loader,
		logger:    logger.NewLogger("controller"),
	}
	ctl.group = group.NewManager(meta, loader)
	return ctl
}
