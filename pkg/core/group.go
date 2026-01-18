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
	"runtime/trace"

	"github.com/basenana/nanafs/pkg/metastore"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/types"
)

type Group interface {
	FindEntry(ctx context.Context, name string) (*types.Entry, error)
	ListChildren(ctx context.Context) ([]*types.Entry, error)
}

type emptyGroup struct{}

var _ Group = emptyGroup{}

func (e emptyGroup) FindEntry(ctx context.Context, name string) (*types.Entry, error) {
	return nil, types.ErrNotFound
}

func (e emptyGroup) ListChildren(ctx context.Context) ([]*types.Entry, error) {
	return make([]*types.Entry, 0), nil
}

type stdGroup struct {
	entryID   int64
	name      string
	namespace string
	core      Core
	store     metastore.EntryStore
}

var _ Group = &stdGroup{}

func (g *stdGroup) FindEntry(ctx context.Context, name string) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.stdGroup.FindEntry").End()
	ch, err := g.core.FindEntry(ctx, g.namespace, g.entryID, name)
	if err != nil {
		return nil, err
	}
	return g.core.GetEntry(ctx, g.namespace, ch.ChildID)
}

func (g *stdGroup) ListChildren(ctx context.Context) ([]*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.stdGroup.ListChildren").End()
	children, err := g.core.ListChildren(ctx, g.namespace, g.entryID)
	if err != nil {
		return nil, err
	}

	var (
		result = make([]*types.Entry, 0, len(children))
		next   *types.Entry
	)
	for _, child := range children {
		next, err = g.core.GetEntry(ctx, g.namespace, child.ChildID)
		if err != nil {
			return nil, err
		}
		result = append(result, next)
	}
	return result, nil
}

type dynamicGroup struct {
	std       *stdGroup
	filter    types.Filter
	baseEntry int64
	logger    *zap.SugaredLogger
}

func (d *dynamicGroup) FindEntry(ctx context.Context, name string) (*types.Entry, error) {
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

func (d *dynamicGroup) ListChildren(ctx context.Context) ([]*types.Entry, error) {
	it, err := d.std.store.FilterEntries(ctx, d.std.namespace, d.filter)
	if err != nil {
		d.logger.Errorw("list static children failed", "err", err)
		return nil, err
	}

	children := make([]*types.Entry, 0)
	for it.HasNext() {
		child, err := it.Next()
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}

	return children, nil
}
