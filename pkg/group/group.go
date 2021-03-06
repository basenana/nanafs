package group

import (
	"context"
	"encoding/json"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/plugin"
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
	result := make([]*types.Object, 0)
	if group.Rule == nil {
		if group.ExtendData.PlugScope != nil && group.ExtendData.PlugScope.PluginType == types.PluginTypeMirror {
			return m.mirroredSmtGroupChildren(ctx, group)
		}
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

func (m *Manager) mirroredSmtGroupChildren(ctx context.Context, group Group) ([]*types.Object, error) {
	p, err := plugin.LoadPlugin(group.ExtendData.PlugScope.PluginName)
	if err != nil {
		m.logger.Errorw("load group plugin failed", "gid", group.ID, "plugin", group.ExtendData.PlugScope.PluginName, "err", err.Error())
		return nil, err
	}

	plug, ok := p.(types.MirrorPlugin)
	if !ok {
		m.logger.Warnw("no mirror plugin", "gid", group.ID, "plugin", group.ExtendData.PlugScope.PluginName)
		return nil, nil
	}

	pathAnn := group.ExtendData.Annotation.Get(types.PluginAnnPath)
	path := pathAnn.Content
	if path == "" {
		path = "/"
	}
	files, err := plug.List(ctx, path, group.ExtendData.PlugScope.Parameters)
	if err != nil {
		m.logger.Errorw("run mirror plugin list method failed", "gid", group.ID, "plugin", group.ExtendData.PlugScope.PluginName, "err", err.Error())
		return nil, err
	}
	// TODO
	_ = files
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
		meta:      meta,
		cfg:       cfg,
		cfgLoader: cfgLoader,
		logger:    logger.NewLogger("groupManager"),
	}
	return mgr
}

type Group struct {
	*types.Object
	*types.Rule
}
