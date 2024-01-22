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

package dentry

import (
	"context"
	"errors"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/rule"
	"github.com/basenana/nanafs/pkg/types"
	"go.uber.org/zap"
	"path"
	"runtime/trace"
)

type Group interface {
	FindEntry(ctx context.Context, name string) (*types.Metadata, error)
	CreateEntry(ctx context.Context, attr types.EntryAttr) (*types.Metadata, error)
	UpdateEntry(ctx context.Context, entry *types.Metadata) error
	RemoveEntry(ctx context.Context, entryID int64) error
	ListChildren(ctx context.Context) ([]*types.Metadata, error)
}

type emptyGroup struct{}

var _ Group = emptyGroup{}

func (e emptyGroup) FindEntry(ctx context.Context, name string) (*types.Metadata, error) {
	return nil, types.ErrNotFound
}

func (e emptyGroup) CreateEntry(ctx context.Context, attr types.EntryAttr) (*types.Metadata, error) {
	return nil, types.ErrNoAccess
}

func (e emptyGroup) UpdateEntry(ctx context.Context, entry *types.Metadata) error {
	return types.ErrNoAccess
}

func (e emptyGroup) RemoveEntry(ctx context.Context, enId int64) error {
	return types.ErrNoAccess
}

func (e emptyGroup) ListChildren(ctx context.Context) ([]*types.Metadata, error) {
	return make([]*types.Metadata, 0), nil
}

type stdGroup struct {
	entryID int64
	name    string
	mgr     *manager
	store   metastore.DEntry
}

var _ Group = &stdGroup{}

func (g *stdGroup) GetEntry(ctx context.Context, entryID int64) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.GetEntry").End()
	entry, err := g.store.GetEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}
	if entry.ParentID != g.entryID {
		return nil, types.ErrNotFound
	}
	return entry, nil
}

func (g *stdGroup) FindEntry(ctx context.Context, name string) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.FindEntry").End()
	entry, err := g.store.FindEntry(ctx, g.entryID, name)
	if err != nil {
		return nil, err
	}
	if entry.Storage == externalStorage {
		return entry, g.mgr.registerStubRoot(ctx, entry)
	}
	return entry, nil
}

func (g *stdGroup) CreateEntry(ctx context.Context, attr types.EntryAttr) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.CreateEntry").End()
	existed, err := g.store.FindEntry(ctx, g.entryID, attr.Name)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}
	if existed != nil {
		return nil, types.ErrIsExist
	}

	group, err := g.store.GetEntry(ctx, g.entryID)
	if err != nil {
		return nil, err
	}

	entry, err := types.InitNewEntry(group, attr)
	if err != nil {
		return nil, err
	}

	if entry.Kind == types.ExternalGroupKind {
		entry.Storage = externalStorage
	}

	err = g.store.CreateEntry(ctx, g.entryID, entry)
	if err != nil {
		return nil, err
	}

	ed := attr.ExtendData
	labels := attr.Labels
	switch entry.Kind {
	case types.ExternalGroupKind:
		if attr.PlugScope != nil {
			if attr.PlugScope.Parameters == nil {
				attr.PlugScope.Parameters = map[string]string{}
			}
			ed.PlugScope = attr.PlugScope
			ed.PlugScope.Parameters[types.PlugScopeEntryName] = attr.Name
			ed.PlugScope.Parameters[types.PlugScopeEntryPath] = "/"
		}

		labels.Labels = append(labels.Labels, []types.Label{
			{Key: types.LabelKeyPluginName, Value: ed.PlugScope.PluginName},
			{Key: types.LabelKeyPluginKind, Value: string(types.TypeMirror)},
		}...)
	case types.SmartGroupKind:
		ed.GroupFilter = attr.GroupFilter
	default:
		// skip create extend data
		g.mgr.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeCreate, entry.ID)
		return entry, nil
	}

	if err = g.store.UpdateEntryExtendData(ctx, entry.ID, ed); err != nil {
		_ = g.store.RemoveEntry(ctx, entry.ParentID, entry.ID)
		return nil, err
	}
	if len(labels.Labels) > 0 {
		if err = g.store.UpdateEntryLabels(ctx, entry.ID, labels); err != nil {
			_ = g.store.RemoveEntry(ctx, entry.ParentID, entry.ID)
			return nil, err
		}
	}

	g.mgr.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeCreate, entry.ID)
	return entry, nil
}

func (g *stdGroup) UpdateEntry(ctx context.Context, entry *types.Metadata) error {
	defer trace.StartRegion(ctx, "dentry.stdGroup.UpdateEntry").End()
	err := g.store.UpdateEntryMetadata(ctx, entry)
	if err != nil {
		return err
	}
	g.mgr.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeUpdate, entry.ID)
	return nil
}

func (g *stdGroup) RemoveEntry(ctx context.Context, entryId int64) error {
	defer trace.StartRegion(ctx, "dentry.stdGroup.RemoveEntry").End()
	en, err := g.store.GetEntry(ctx, entryId)
	if err != nil {
		return err
	}
	if en.ParentID != g.entryID {
		return types.ErrNotFound
	}
	err = g.store.RemoveEntry(ctx, g.entryID, entryId)
	if err != nil {
		return err
	}
	g.mgr.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeDestroy, entryId)
	return nil
}

func (g *stdGroup) ListChildren(ctx context.Context) ([]*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.ListChildren").End()
	it, err := g.store.ListEntryChildren(ctx, g.entryID)
	if err != nil {
		return nil, err
	}
	var (
		result = make([]*types.Metadata, 0)
		next   *types.Metadata
	)
	for it.HasNext() {
		next = it.Next()
		if next.ID == next.ParentID {
			continue
		}
		result = append(result, next)
	}
	return result, nil
}

type dynamicGroup struct {
	std       *stdGroup
	rule      types.Rule
	baseEntry int64
	logger    *zap.SugaredLogger
}

func (d *dynamicGroup) FindEntry(ctx context.Context, name string) (*types.Metadata, error) {
	children, err := d.ListChildren(ctx)
	if err != nil {
		return nil, err
	}
	for i, ch := range children {
		if ch.Name == name {
			return children[i], nil
		}
	}
	return nil, types.ErrNotFound
}

func (d *dynamicGroup) CreateEntry(ctx context.Context, attr types.EntryAttr) (*types.Metadata, error) {
	return d.std.CreateEntry(ctx, attr)
}

func (d *dynamicGroup) UpdateEntry(ctx context.Context, entry *types.Metadata) error {
	return d.std.UpdateEntry(ctx, entry)
}

func (d *dynamicGroup) RemoveEntry(ctx context.Context, entryID int64) error {
	_, err := d.std.GetEntry(ctx, entryID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			// can not delete auto selected entry
			return types.ErrNoPerm
		}
	}
	return d.std.RemoveEntry(ctx, entryID)
}

func (d *dynamicGroup) ListChildren(ctx context.Context) ([]*types.Metadata, error) {
	children, err := d.std.ListChildren(ctx)
	if err != nil {
		d.logger.Errorw("list static children failed", "err", err)
		return nil, err
	}
	childrenMap := map[string]struct{}{}
	for _, ch := range children {
		childrenMap[ch.Name] = struct{}{}
	}

	dynamicChildren, err := rule.Q().Rule(d.rule).Results(ctx)
	if err != nil {
		d.logger.Errorw("list children with rule failed", "err", err)
		return nil, err
	}

	for i, ch := range dynamicChildren {
		if _, ok := childrenMap[ch.Name]; ok {
			continue
		}
		children = append(children, dynamicChildren[i])
	}
	return children, nil
}

type extGroup struct {
	entry  *StubEntry
	mirror plugin.MirrorPlugin
	logger *zap.SugaredLogger
}

func (e *extGroup) FindEntry(ctx context.Context, name string) (*types.Metadata, error) {
	children, err := e.ListChildren(ctx)
	if err != nil {
		return nil, err
	}
	for i, ch := range children {
		if ch.Name == name {
			return children[i], nil
		}
	}
	return nil, types.ErrNotFound
}

func (e *extGroup) CreateEntry(ctx context.Context, attr types.EntryAttr) (*types.Metadata, error) {
	mirrorEn, err := e.mirror.CreateEntry(ctx, e.entry.path, pluginapi.EntryAttr{
		Name: attr.Name,
		Kind: attr.Kind,
	})
	if err != nil {
		return nil, err
	}
	return e.entry.createChild(mirrorEn)
}

func (e *extGroup) UpdateEntry(ctx context.Context, entry *types.Metadata) error {
	entryPath := path.Join(e.entry.path, entry.Name)

	// query old and write back
	mirrorEn, err := e.mirror.GetEntry(ctx, entryPath)
	if err != nil {
		e.logger.Warnw("find entry in mirror plugin failed", "name", entry.Name, "err", err)
		return err
	}

	mirrorEn.Size = entry.Size
	err = e.mirror.UpdateEntry(ctx, entryPath, mirrorEn)
	if err != nil {
		e.logger.Warnw("update entry to mirror plugin failed", "name", entry.Name, "err", err)
		return err
	}
	err = e.entry.updateChild(mirrorEn)
	if err != nil {
		e.logger.Warnw("update ext entry indexer failed", "name", entry.Name, "err", err)
		return err
	}
	return nil
}

func (e *extGroup) RemoveEntry(ctx context.Context, entryId int64) error {
	child, err := e.entry.root.GetStubEntry(entryId)
	if err != nil {
		return err
	}
	if child.parent != e.entry.id {
		return types.ErrNotFound
	}

	err = e.mirror.RemoveEntry(ctx, child.path)
	if err != nil {
		return err
	}

	return e.entry.removeChild(entryId)
}

func (e *extGroup) ListChildren(ctx context.Context) ([]*types.Metadata, error) {
	actualChild, err := e.mirror.ListChildren(ctx, e.entry.path)
	if err != nil {
		return nil, err
	}
	recordChild := e.entry.listChildren()

	recordChildMap := make(map[string]*types.Metadata)
	actualChildMap := make(map[string]*pluginapi.Entry)
	for i, ch := range recordChild {
		recordChildMap[ch.Name] = recordChild[i]
	}
	for i, ch := range actualChild {
		actualChildMap[ch.Name] = actualChild[i]
	}

	result := make([]*types.Metadata, 0, len(actualChild))
	for k := range actualChildMap {
		record := recordChildMap[k]
		if record == nil {
			// create
			en, err := e.entry.createChild(actualChildMap[k])
			if err != nil {
				return nil, err
			}
			result = append(result, en)
			continue
		}

		// update
		err = e.entry.updateChild(actualChildMap[k])
		if err != nil {
			return nil, err
		}

		en, err := e.entry.findChild(actualChildMap[k].Name)
		if err != nil {
			return nil, err
		}
		result = append(result, en)
		delete(recordChildMap, k)
	}

	for _, ch := range recordChildMap {
		e.entry.root.RemoveStubEntry(ch.ID)
	}
	return result, nil
}
