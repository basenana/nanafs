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
func InitSchemas(ctrl Controller) error {
	schemas := dentry.Registry.GetSchemas()
	root, err := ctrl.LoadRootObject(context.TODO())
	if err != nil {
		return err
	}
	for _, s := range schemas {
		obj, _ := types.InitNewObject(nil, types.ObjectAttr{Name: fmt.Sprintf(".%s", string(s.CType)), Kind: types.GroupKind})
		_, err = ctrl.FindObject(context.TODO(), root, obj.Name)
		if err != nil && err != types.ErrNotFound {
			return err
		}
		if err == nil {
			continue
		}
		obj.ParentID = root.ID
		obj.Labels = types.Labels{Labels: []types.Label{{
			Key:   types.VersionKey,
			Value: s.Version,
		}, {
			Key:   types.KindKey,
			Value: string(s.CType),
		}}}
		if err = ctrl.SaveObject(context.TODO(), obj); err != nil {
			return err
		}
	}
	return nil
}
