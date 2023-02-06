package group

import (
	"context"
	"encoding/json"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

const (
	smtGroupAnnKey = "smt.rule.filter"
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
		filtered, err := m.filterSmtGroupChildren(ctx, Group{Object: obj, Rule: obj.ExtendData.GroupFilter})
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
	return nil, nil
}

func (m *Manager) UpdateGroupFilterRule(ctx context.Context, obj *types.Object, rule *types.Rule) error {
	if !obj.IsGroup() {
		return types.ErrNoGroup
	}

	if err := validateRuleSpec(rule); err != nil {
		m.logger.Errorw("rule not valid", "gid", obj.ID, "err", err.Error())
		return err
	}

	if !obj.IsSmartGroup() {
		children, err := m.meta.ListChildren(ctx, obj)
		if err != nil {
			m.logger.Errorw("check group children failed", "gid", obj.ID, "err", err.Error())
			return err
		}

		if children.HasNext() {
			m.logger.Errorw("group has children", "gid", obj.ID)
			return types.ErrNotEmpty
		}

		obj.Kind = types.SmartGroupKind
	}

	obj.ExtendData.GroupFilter = rule
	err := m.meta.SaveObject(ctx, nil, obj)
	if err != nil {
		m.logger.Errorw("save group config failed", "gid", obj.ID, "err", err.Error())
		return err
	}
	return nil
}

func (m *Manager) loadFilterConfig(ctx context.Context, obj *types.Object) (*types.Rule, error) {
	ann := dentry.GetInternalAnnotation(obj, smtGroupAnnKey)
	if ann == nil {
		return nil, nil
	}

	rule := &types.Rule{}
	if err := json.Unmarshal([]byte(ann.Content), rule); err != nil {
		return nil, err
	}

	return rule, nil
}

func NewManager(meta storage.MetaStore, cfgLoader config.Loader) *Manager {
	cfg, _ := cfgLoader.GetConfig()

	mgr := &Manager{
		meta:   meta,
		cfg:    cfg,
		logger: logger.NewLogger("groupManager"),
	}
	return mgr
}

type Group struct {
	*types.Object
	*types.Rule
}
