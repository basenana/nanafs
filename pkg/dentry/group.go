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
	"fmt"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
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
	store   metastore.DEntry
}

var _ Group = &stdGroup{}

func (g *stdGroup) GetEntry(ctx context.Context, entryID int64) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.GetEntry").End()
	entry, err := g.store.GetEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (g *stdGroup) FindEntry(ctx context.Context, name string) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.FindEntry").End()
	entry, err := g.store.FindEntry(ctx, g.entryID, name)
	if err != nil {
		return nil, err
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

	entry, err := types.InitNewEntry(group, types.EntryAttr{
		Name:   attr.Name,
		Kind:   attr.Kind,
		Access: attr.Access,
		Dev:    attr.Dev,
	})
	if err != nil {
		return nil, err
	}

	err = g.store.CreateEntry(ctx, g.entryID, entry)
	if err != nil {
		return nil, err
	}

	ed := attr.ExtendData
	labels := attr.Labels
	switch entry.Kind {
	case types.ExternalGroupKind:
		entry.Storage = externalStorage
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
	return entry, nil
}

func (g *stdGroup) UpdateEntry(ctx context.Context, entry *types.Metadata) error {
	defer trace.StartRegion(ctx, "dentry.stdGroup.UpdateEntry").End()
	return g.store.UpdateEntryMetadata(ctx, entry)
}

func (g *stdGroup) RemoveEntry(ctx context.Context, entryId int64) error {
	defer trace.StartRegion(ctx, "dentry.stdGroup.RemoveEntry").End()
	err := g.store.RemoveEntry(ctx, g.entryID, entryId)
	if err != nil {
		return err
	}
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
	*stdGroup
}

type extGroup struct {
	mgr      Manager
	stdGroup *stdGroup
	mirror   plugin.MirrorPlugin
	logger   *zap.SugaredLogger
}

func (e *extGroup) FindEntry(ctx context.Context, name string) (*types.Metadata, error) {
	mirrorEn, err := e.mirror.FindEntry(ctx, name)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}

	en, err := e.stdGroup.FindEntry(ctx, name)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}
	return e.syncEntry(ctx, mirrorEn, en)
}

func (e *extGroup) CreateEntry(ctx context.Context, attr types.EntryAttr) (*types.Metadata, error) {
	mirrorEn, err := e.mirror.CreateEntry(ctx, pluginapi.EntryAttr{
		Name: attr.Name,
		Kind: attr.Kind,
	})
	if err != nil {
		return nil, err
	}
	en, err := e.stdGroup.FindEntry(ctx, attr.Name)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}
	return e.syncEntry(ctx, mirrorEn, en)
}

func (e *extGroup) UpdateEntry(ctx context.Context, entry *types.Metadata) error {
	// query old and write back
	entry, err := e.stdGroup.GetEntry(ctx, entry.ID)
	if err != nil {
		return err
	}

	mirrorEn, err := e.mirror.FindEntry(ctx, entry.Name)
	if err != nil {
		e.logger.Warnw("find entry in mirror plugin failed", "name", entry.Name, "err", err)
		return err
	}

	mirrorEn.Size = entry.Size

	err = e.mirror.UpdateEntry(ctx, mirrorEn)
	if err != nil {
		e.logger.Warnw("update entry to mirror plugin failed", "name", entry.Name, "err", err)
		return err
	}

	_, err = e.syncEntry(ctx, mirrorEn, entry)
	return err
}

func (e *extGroup) RemoveEntry(ctx context.Context, entryId int64) error {
	entry, err := e.stdGroup.GetEntry(ctx, entryId)
	if err != nil {
		return err
	}

	mirrorEn, err := e.mirror.FindEntry(ctx, entry.Name)
	if err != nil {
		return err
	}

	err = e.mirror.RemoveEntry(ctx, mirrorEn)
	if err != nil {
		return err
	}

	_, err = e.syncEntry(ctx, nil, entry)
	if err != nil {
		if err == types.ErrNotFound {
			return nil
		}
		return err
	}
	return nil
}

func (e *extGroup) ListChildren(ctx context.Context) ([]*types.Metadata, error) {
	recordChild, err := e.stdGroup.ListChildren(ctx)
	if err != nil {
		return nil, err
	}
	actualChild, err := e.mirror.ListChildren(ctx)
	if err != nil {
		return nil, err
	}

	recordChildMap := make(map[string]*types.Metadata)
	actualChildMap := make(map[string]*pluginapi.Entry)
	for i := range recordChild {
		recordChildMap[recordChild[i].Name] = recordChild[i]
	}
	for i := range actualChild {
		actualChildMap[actualChild[i].Name] = actualChild[i]
	}

	result := make([]*types.Metadata, 0, len(actualChild))
	for k := range actualChildMap {
		en, err := e.syncEntry(ctx, actualChildMap[k], recordChildMap[k])
		if err != nil && err != types.ErrNotFound {
			return nil, err
		}
		if en != nil {
			result = append(result, en)
		}
		delete(actualChildMap, k)
		delete(recordChildMap, k)
	}

	for k := range recordChildMap {
		_, err = e.syncEntry(ctx, nil, recordChildMap[k])
		if err != nil && err != types.ErrNotFound {
			return nil, err
		}
	}
	return result, nil
}

func (e *extGroup) syncEntry(ctx context.Context, mirrored *pluginapi.Entry, crt *types.Metadata) (en *types.Metadata, err error) {
	grp, err := e.stdGroup.GetEntry(ctx, e.stdGroup.entryID)
	if err != nil {
		return nil, err
	}
	grpEd, err := e.mgr.GetEntryExtendData(ctx, e.stdGroup.entryID)
	if err != nil {
		return
	}
	if grpEd.PlugScope == nil {
		err = fmt.Errorf("not ext group")
		return
	}

	if mirrored != nil && crt != nil && mirrored.IsGroup != types.IsGroup(crt.Kind) {
		// FIXME
		return nil, fmt.Errorf("entry and mirrored entry incorrect")
	}

	if mirrored == nil {
		err = types.ErrNotFound
		if crt != nil {
			// clean
			_ = e.stdGroup.store.RemoveEntry(ctx, grp.ID, crt.ID)
		}
		return nil, err
	}

	if crt == nil {
		// create mirror record
		var (
			newEn   *types.Metadata
			newEnEd types.ExtendData
		)
		newEn, err = types.InitNewEntry(grp, types.EntryAttr{
			Name:   mirrored.Name,
			Kind:   mirrored.Kind,
			Access: grp.Access,
		})
		if err != nil {
			return nil, err
		}

		newEn.Storage = externalStorage

		newEnEd.PlugScope = &types.PlugScope{
			PluginName: grpEd.PlugScope.PluginName,
			Version:    grpEd.PlugScope.Version,
			PluginType: grpEd.PlugScope.PluginType,
			Parameters: map[string]string{},
		}
		if mirrored.Parameters != nil {
			for k, v := range mirrored.Parameters {
				newEnEd.PlugScope.Parameters[k] = v
			}
		}
		for k, v := range grpEd.PlugScope.Parameters {
			newEnEd.PlugScope.Parameters[k] = v
		}
		newEnEd.PlugScope.Parameters[types.PlugScopeEntryName] = mirrored.Name
		newEnEd.PlugScope.Parameters[types.PlugScopeEntryPath] = path.Join(grpEd.PlugScope.Parameters[types.PlugScopeEntryPath], mirrored.Name)

		if err = e.stdGroup.store.CreateEntry(ctx, grp.ID, newEn); err != nil {
			return nil, err
		}

		if err = e.stdGroup.store.UpdateEntryExtendData(ctx, newEn.ID, newEnEd); err != nil {
			_ = e.stdGroup.store.RemoveEntry(ctx, grp.ID, newEn.ID)
			return nil, err
		}
		en = newEn
		return
	}

	// update mirror record
	crt.Size = mirrored.Size
	if err = e.stdGroup.store.UpdateEntryMetadata(ctx, crt); err != nil {
		return nil, err
	}
	en = crt
	return
}
