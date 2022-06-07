package group

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

type Manager struct {
	meta      storage.MetaStore
	cfg       config.Config
	cfgLoader config.Loader
	logger    *zap.SugaredLogger
}

func (m *Manager) ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error) {
	if !obj.IsGroup() {
		return nil, types.ErrNoGroup
	}

	if obj.IsSmartGroup() {
		return m.filterSmtGroupChildren(ctx, Group{Object: obj})
	}

	return m.listGroupChildren(ctx, Group{Object: obj})
}

func (m *Manager) listGroupChildren(ctx context.Context, group Group) ([]*types.Object, error) {
	defer utils.TraceRegion(ctx, "group.list")()
	m.logger.Infow("list children obj", "gid", group.ID)
	result := make([]*types.Object, 0)
	it, err := m.meta.ListChildren(ctx, group.Object)
	if err != nil {
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

func (m *Manager) filterSmtGroupChildren(ctx context.Context, group Group) ([]*types.Object, error) {
	defer utils.TraceRegion(ctx, "group.list")()
	m.logger.Infow("list smart group children obj", "gid", group.ID)
	return nil, nil
}

func NewManager(meta storage.MetaStore, cfgLoader config.Loader) *Manager {
	cfg, _ := cfgLoader.GetConfig()

	return &Manager{
		meta:      meta,
		cfg:       cfg,
		cfgLoader: cfgLoader,
		logger:    logger.NewLogger("groupManager"),
	}
}

type Group struct {
	*types.Object
	*types.Rule
}
