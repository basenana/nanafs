package group

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hyponet/eventbus/bus"
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
	result, err := m.listGroupChildren(ctx, Group{Object: obj})
	if err != nil {
		return nil, err
	}
	if obj.IsSmartGroup() {
		rule, err := m.loadFilterConfig(ctx, obj)
		if err != nil {
			return nil, err
		}

		filtered, err := m.filterSmtGroupChildren(ctx, Group{Object: obj, Rule: rule})
		if err != nil {
			return nil, err
		}
		result = append(result, filtered...)
	}
	return result, nil
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
	result := make([]*types.Object, 0)
	if group.Rule == nil {
		return result, nil
	}

	defer utils.TraceRegion(ctx, "group.filter")()
	m.logger.Infow("list smart group children obj", "gid", group.ID)
	labelSelector := group.Selector
	if len(labelSelector.Include) == 0 && len(labelSelector.Exclude) == 0 {
		m.logger.Warnf("group has no label selector, interrupt")
		return result, nil
	}

	rawList, err := m.meta.ListObjects(ctx, types.Filter{Label: labelSelector})
	if err != nil {
		m.logger.Errorw("group selector object with label selector failed", "gid", group.ID, "err", err.Error())
		return nil, err
	}

	for i, obj := range rawList {
		if RuleMatch(group.Rule, obj) {
			result = append(result, rawList[i])
		}
	}
	return result, nil
}

func (m *Manager) loadFilterConfig(ctx context.Context, groupObj *types.Object) (*types.Rule, error) {
	return nil, nil
}

func (m *Manager) groupFilterHandle(obj *types.Object) {
	if obj.Name != groupFilterRuleFile {
		return
	}

	m.logger.Infow("handle group filter config", "obj", obj.ID, "group", obj.ParentID)
	dirObj, err := m.meta.GetObject(context.Background(), obj.ParentID)
	if err != nil {
		m.logger.Errorw("query group object failed", "obj", obj.ID, "group", obj.ParentID, "err", err.Error())
		return
	}

	_, err = m.loadFilterConfig(context.Background(), dirObj)
	if err != nil {
		m.logger.Errorw("load group filter config failed", "obj", obj.ID, "group", obj.ParentID, "err", err.Error())
		return
	}

	if !dirObj.IsSmartGroup() {
		// migrate group to smt group
		dirObj.Kind = types.SmartGroupKind
		if err = m.meta.SaveObject(context.Background(), nil, obj); err != nil {
			m.logger.Errorw("migrate group object to smt group failed", "obj", obj.ID, "group", obj.ParentID, "err", err.Error())
			return
		}
	}
}

func NewManager(meta storage.MetaStore, cfgLoader config.Loader) *Manager {
	cfg, _ := cfgLoader.GetConfig()

	mgr := &Manager{
		meta:      meta,
		cfg:       cfg,
		cfgLoader: cfgLoader,
		logger:    logger.NewLogger("groupManager"),
	}

	_, _ = bus.Subscribe("object.file.*.close", mgr.groupFilterHandle)

	return mgr
}

type Group struct {
	*types.Object
	*types.Rule
}
