package group

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"go.uber.org/zap"
)

type Manager struct {
	meta      storage.MetaStore
	storage   storage.Storage
	cfg       config.Config
	cfgLoader config.Loader
	logger    *zap.SugaredLogger
}

func (m *Manager) ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error) {
	return nil, nil
}

func (m *Manager) listGroupChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error) {
	return nil, nil
}

func (m *Manager) listGroupChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error) {
	return nil, nil
}

func NewManager() *Manager {
	return &Manager{}
}
