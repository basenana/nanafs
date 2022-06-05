package group

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
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
	if !obj.IsGroup() {
		return nil, types.ErrNoGroup
	}

	if obj.IsSmartGroup() {
		return m.filterGroupChildren(ctx, Group{Object: obj})
	}

	return m.listGroupChildren(ctx, Group{Object: obj})
}

func (m *Manager) listGroupChildren(ctx context.Context, group Group) ([]*types.Object, error) {
	defer utils.TraceRegion(ctx, "group.list")()
	m.logger.Infow("list children obj", "name", group.Name)
	result := make([]*types.Object, 0)
	it, err := m.meta.ListChildren(ctx, group.Object)
	if err != nil {
		m.logger.Errorw("load object children error", "group", group.ID, "err", err.Error())
		return nil, err
	}
	for it.HasNext() {
		next := it.Next()
		if next.ID == next.ParentID {
			continue
		}
		result = append(result, next)
	}
	return result, nil
}

func (m *Manager) filterGroupChildren(ctx context.Context, group Group) ([]*types.Object, error) {
	return nil, nil
}

func NewManager() *Manager {
	return &Manager{}
}

type Group struct {
	*types.Object
	Rule
}
