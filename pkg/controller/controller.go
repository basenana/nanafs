/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package controller

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/group"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

type Controller interface {
	LoadRootObject(ctx context.Context) (*types.Object, error)
	FindObject(ctx context.Context, parent *types.Object, name string) (*types.Object, error)
	GetObject(ctx context.Context, id int64) (*types.Object, error)
	CreateObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	SaveObject(ctx context.Context, parent, obj *types.Object) error
	DestroyObject(ctx context.Context, parent, obj *types.Object, attr types.DestroyObjectAttr) error
	MirrorObject(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error)
	ChangeObjectParent(ctx context.Context, old, oldParent, newParent *types.Object, newName string, opt types.ChangeParentAttr) error

	OpenFile(ctx context.Context, obj *types.Object, attr files.Attr) (files.File, error)
	ReadFile(ctx context.Context, file files.File, data []byte, offset int64) (n int, err error)
	WriteFile(ctx context.Context, file files.File, data []byte, offset int64) (n int64, err error)
	CloseFile(ctx context.Context, file files.File) error
	DeleteFileData(ctx context.Context, obj *types.Object) error

	FsInfo(ctx context.Context) Info
}

type controller struct {
	meta      storage.Meta
	storage   storage.Storage
	cfg       config.Config
	cfgLoader config.Loader

	group *group.Manager

	logger *zap.SugaredLogger
}

var _ Controller = &controller{}

func New(loader config.Loader, meta storage.Meta, storage storage.Storage) Controller {
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
