package controller

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

const (
	workflowGroup = ".workflow"
	jobGroup      = ".job"
)

type Controller interface {
	ObjectController
	FileController
	FsController
	StructuredController
	WorkflowController
}

type controller struct {
	meta      storage.MetaStore
	storage   storage.Storage
	cfgLoader config.Loader
	logger    *zap.SugaredLogger
	registry  *dentry.SchemaRegistry
}

var _ Controller = &controller{}

func New(loader config.Loader, meta storage.MetaStore, storage storage.Storage) Controller {
	ctl := &controller{
		meta:      meta,
		storage:   storage,
		cfgLoader: loader,
		logger:    logger.NewLogger("controller"),
		registry:  dentry.Registry,
	}
	return ctl
}

func (c *controller) setup(ctx context.Context) error {
	for _, s := range c.registry.GetSchemas() {
		root, _ := types.InitNewObject(nil, types.ObjectAttr{Name: fmt.Sprintf(".%s", string(s.CType)), Kind: types.GroupKind})
		root.ParentID = dentry.RootObjectID
		root.Labels = types.Labels{Labels: []types.Label{{
			Key:   types.VersionKey,
			Value: s.Version,
		}, {
			Key:   types.KindKey,
			Value: string(s.CType),
		}}}
		if err := c.SaveObject(ctx, root); err != nil {
			c.logger.Infow("save object error", "err", err)
			return err
		}
	}
	return nil
}
