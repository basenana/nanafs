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

package core

import (
	"context"
	"errors"
	"runtime/trace"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/rule"
	"github.com/basenana/nanafs/pkg/types"
)

type Group interface {
	FindEntry(ctx context.Context, name string) (*types.Entry, error)
	CreateEntry(ctx context.Context, attr types.EntryAttr) (*types.Entry, error)
	UpdateEntry(ctx context.Context, entry *types.Entry) error
	RemoveEntry(ctx context.Context, entryID int64) error
	ListChildren(ctx context.Context, order *types.EntryOrder, filters ...types.Filter) ([]*types.Entry, error)
}

type emptyGroup struct{}

var _ Group = emptyGroup{}

func (e emptyGroup) FindEntry(ctx context.Context, name string) (*types.Entry, error) {
	return nil, types.ErrNotFound
}

func (e emptyGroup) CreateEntry(ctx context.Context, attr types.EntryAttr) (*types.Entry, error) {
	return nil, types.ErrNoAccess
}

func (e emptyGroup) UpdateEntry(ctx context.Context, entry *types.Entry) error {
	return types.ErrNoAccess
}

func (e emptyGroup) RemoveEntry(ctx context.Context, enId int64) error {
	return types.ErrNoAccess
}

func (e emptyGroup) ListChildren(ctx context.Context, order *types.EntryOrder, filters ...types.Filter) ([]*types.Entry, error) {
	return make([]*types.Entry, 0), nil
}

type stdGroup struct {
	entryID   int64
	name      string
	namespace string
	store     metastore.EntryStore
}

var _ Group = &stdGroup{}

func (g *stdGroup) GetEntry(ctx context.Context, entryID int64) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.stdGroup.GetEntry").End()
	entry, err := g.store.GetEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}
	if entry.ParentID != g.entryID {
		return nil, types.ErrNotFound
	}
	return entry, nil
}

func (g *stdGroup) FindEntry(ctx context.Context, name string) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.stdGroup.FindEntry").End()
	entry, err := g.store.FindEntry(ctx, g.entryID, name)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (g *stdGroup) CreateEntry(ctx context.Context, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.stdGroup.CreateEntry").End()
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

	err = g.store.CreateEntry(ctx, g.entryID, entry, attr.ExtendData)
	if err != nil {
		return nil, err
	}

	if len(attr.Labels.Labels) > 0 {
		if err = g.store.UpdateEntryLabels(ctx, entry.ID, attr.Labels); err != nil {
			_ = g.store.RemoveEntry(ctx, entry.ParentID, entry.ID)
			return nil, err
		}
	}

	if len(attr.Properties.Fields) > 0 {
		if err = g.store.UpdateEntryProperties(ctx, entry.ID, attr.Properties); err != nil {
			_ = g.store.RemoveEntry(ctx, entry.ParentID, entry.ID)
			return nil, err
		}
	}

	publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeCreate, entry.ID)
	return entry, nil
}

func (g *stdGroup) UpdateEntry(ctx context.Context, entry *types.Entry) error {
	defer trace.StartRegion(ctx, "fs.core.stdGroup.UpdateEntry").End()
	err := g.store.UpdateEntryMetadata(ctx, entry)
	if err != nil {
		return err
	}
	publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeUpdate, entry.ID)
	return nil
}

func (g *stdGroup) RemoveEntry(ctx context.Context, entryId int64) error {
	defer trace.StartRegion(ctx, "fs.core.stdGroup.RemoveEntry").End()
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
	publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeDestroy, entryId)
	return nil
}

func (g *stdGroup) ListChildren(ctx context.Context, order *types.EntryOrder, filters ...types.Filter) ([]*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.stdGroup.ListChildren").End()
	it, err := g.store.ListEntryChildren(ctx, g.entryID, order, filters...)
	if err != nil {
		return nil, err
	}
	var (
		result = make([]*types.Entry, 0)
		next   *types.Entry
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

func (d *dynamicGroup) FindEntry(ctx context.Context, name string) (*types.Entry, error) {
	children, err := d.ListChildren(ctx, nil, types.Filter{})
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

func (d *dynamicGroup) CreateEntry(ctx context.Context, attr types.EntryAttr) (*types.Entry, error) {
	return d.std.CreateEntry(ctx, attr)
}

func (d *dynamicGroup) UpdateEntry(ctx context.Context, entry *types.Entry) error {
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

func (d *dynamicGroup) ListChildren(ctx context.Context, order *types.EntryOrder, filters ...types.Filter) ([]*types.Entry, error) {
	children, err := d.std.ListChildren(ctx, order, filters...)
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
