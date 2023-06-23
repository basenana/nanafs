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
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/rule"
	"github.com/basenana/nanafs/pkg/types"
	"runtime/trace"
	"sync"
	"time"
)

const (
	RootEntryID   = 1
	RootEntryName = "root"
)

type Entry interface {
	Metadata() *types.Metadata
	GetExtendData(ctx context.Context) (types.ExtendData, error)
	UpdateExtendData(ctx context.Context, ed types.ExtendData) error
	GetExtendField(ctx context.Context, fKey string) (*string, error)
	SetExtendField(ctx context.Context, fKey, fVal string) error
	RemoveExtendField(ctx context.Context, fKey string) error
	RuleMatched(ctx context.Context, ruleSpec types.Rule) bool
	IsGroup() bool
	IsMirror() bool
	Group() Group
}

func buildEntry(obj *types.Object, store metastore.ObjectStore) Entry {
	return &rawEntry{obj: obj, store: store}
}

type rawEntry struct {
	obj   *types.Object
	mux   sync.Mutex
	store metastore.ObjectStore
}

func (r *rawEntry) Metadata() *types.Metadata {
	return &r.obj.Metadata
}

func (r *rawEntry) GetExtendField(ctx context.Context, fKey string) (*string, error) {
	defer trace.StartRegion(ctx, "dentry.rawEntry.GetExtendField").End()
	ed, err := r.GetExtendData(ctx)
	if err != nil {
		return nil, err
	}
	r.mux.Lock()
	defer r.mux.Unlock()
	if ed.Properties.Fields == nil {
		return nil, nil
	}
	fVal, ok := ed.Properties.Fields[fKey]
	if !ok {
		return nil, nil
	}
	return &fVal, nil
}

func (r *rawEntry) SetExtendField(ctx context.Context, fKey, fVal string) error {
	defer trace.StartRegion(ctx, "dentry.rawEntry.SetExtendField").End()
	ed, err := r.GetExtendData(ctx)
	if err != nil {
		return err
	}

	r.mux.Lock()
	if ed.Properties.Fields == nil {
		r.obj.Properties.Fields = map[string]string{}
	}
	ed.Properties.Fields[fKey] = fVal
	r.mux.Unlock()

	return r.UpdateExtendData(ctx, ed)
}

func (r *rawEntry) RemoveExtendField(ctx context.Context, fKey string) error {
	defer trace.StartRegion(ctx, "dentry.rawEntry.RemoveExtendField").End()
	ed, err := r.GetExtendData(ctx)
	if err != nil {
		return err
	}

	r.mux.Lock()
	if ed.Properties.Fields == nil {
		ed.Properties.Fields = map[string]string{}
	}
	_, ok := ed.Properties.Fields[fKey]
	if ok {
		delete(ed.Properties.Fields, fKey)
	}
	r.mux.Unlock()
	if !ok {
		return types.ErrNotFound
	}
	return r.UpdateExtendData(ctx, ed)
}

func (r *rawEntry) GetExtendData(ctx context.Context) (types.ExtendData, error) {
	defer trace.StartRegion(ctx, "dentry.rawEntry.GetExtendData").End()
	if r.obj.ExtendData != nil {
		return *r.obj.ExtendData, nil
	}
	err := r.store.GetObjectExtendData(ctx, r.obj)
	if err != nil {
		return types.ExtendData{}, err
	}
	return *r.obj.ExtendData, nil
}

func (r *rawEntry) UpdateExtendData(ctx context.Context, ed types.ExtendData) error {
	defer trace.StartRegion(ctx, "dentry.rawEntry.UpdateExtendData").End()
	r.obj.ChangedAt = time.Now()
	r.obj.ExtendDataChanged = true
	r.obj.ExtendData = &ed
	return r.store.SaveObjects(ctx, r.obj)
}

func (r *rawEntry) RuleMatched(ctx context.Context, ruleSpec types.Rule) bool {
	if r.obj.ExtendData == nil {
		_, err := r.GetExtendData(ctx)
		if err != nil {
			return false
		}
	}
	// TODO: fetch labels
	return rule.ObjectFilter(ruleSpec, &r.obj.Metadata, r.obj.ExtendData, r.obj.Labels)
}

func (r *rawEntry) IsGroup() bool {
	switch r.obj.Kind {
	case types.GroupKind, types.SmartGroupKind, types.MirrorGroupKind:
		return true
	default:
		return false
	}
}

func (r *rawEntry) IsMirror() bool {
	return !r.obj.IsGroup() && r.obj.RefID != 0 && r.obj.RefID != r.obj.ID
}

func (r *rawEntry) Group() Group {
	if r.IsGroup() {
		grp := &stdGroup{
			Entry: r,
			store: r.store,
		}
		switch r.obj.Kind {
		case types.GroupKind:
			return grp
		case types.SmartGroupKind:
			return &dynamicGroup{stdGroup: grp}
		case types.MirrorGroupKind:
			return &mirroredGroup{stdGroup: grp}
		}
	}
	return nil
}

func initRootEntryObject() *types.Object {
	acc := types.Access{
		Permissions: []types.Permission{
			types.PermOwnerRead,
			types.PermOwnerWrite,
			types.PermOwnerExec,
			types.PermGroupRead,
			types.PermGroupWrite,
			types.PermOthersRead,
		},
	}
	root, _ := types.InitNewObject(nil, types.ObjectAttr{Name: RootEntryName, Kind: types.GroupKind, Access: acc})
	root.ID = RootEntryID
	root.ParentID = root.ID
	return root
}

func initMirrorEntryObject(src, newParent *types.Metadata, attr EntryAttr) (*types.Object, error) {
	obj, err := types.InitNewObject(newParent, types.ObjectAttr{
		Name:   attr.Name,
		Dev:    attr.Dev,
		Kind:   attr.Kind,
		Access: attr.Access,
	})
	if err != nil {
		return nil, err
	}

	obj.Metadata.Kind = src.Kind
	obj.Metadata.Namespace = src.Namespace
	obj.RefID = src.ID
	return obj, nil
}
