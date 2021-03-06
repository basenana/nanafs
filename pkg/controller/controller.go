package controller

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/group"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
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
		registry:  dentry.Registry,
	}

	ctl.group = group.NewManager(meta, loader)
	return ctl
}

func InitSchemas(ctrl Controller, cfg config.Config) error {
	schemas := dentry.Registry.GetSchemas()
	root, err := ctrl.LoadRootObject(context.TODO())
	if err != nil {
		return err
	}

	acc := types.Access{
		Permissions: []types.Permission{
			types.PermOwnerRead,
			types.PermOwnerWrite,
			types.PermOwnerExec,
			types.PermGroupRead,
			types.PermGroupWrite,
			types.PermOthersRead,
		},
		UID: cfg.Owner.Uid,
		GID: cfg.Owner.Gid,
	}

	for _, s := range schemas {
		name := fmt.Sprintf(".%s", string(s.CType))
		_, err = ctrl.FindObject(context.TODO(), root, name)
		if err != nil && err != types.ErrNotFound {
			return err
		}
		if err == nil {
			continue
		}
		obj, err := ctrl.CreateObject(context.Background(), root, types.ObjectAttr{Name: name, Kind: types.GroupKind, Access: acc})
		if err != nil {
			continue
		}
		obj.Labels = types.Labels{Labels: []types.Label{{
			Key:   types.VersionKey,
			Value: s.Version,
		}, {
			Key:   types.KindKey,
			Value: string(s.CType),
		}}}
		if err = ctrl.SaveObject(context.TODO(), root, obj); err != nil {
			_ = ctrl.DestroyObject(context.Background(), root, obj, types.DestroyObjectAttr{})
			return err
		}
	}
	return nil
}
