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
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	RootEntryID   = 1
	RootEntryName = "root"
)

type Entry interface {
	// TODO: delete
	Object() *types.Object

	Metadata() *types.Metadata
	ExtendData() *types.ExtendData
	GetExtendField(fKey string) *string
	SetExtendField(fKey, fVal string)
	RemoveExtendField(fKey string) error
	IsGroup() bool
	IsMirror() bool
	Group() Group
}

func BuildEntry(obj *types.Object, store storage.ObjectStore) Entry {
	return &rawEntry{obj: obj, store: store}
}

type rawEntry struct {
	obj   *types.Object
	store storage.ObjectStore
}

func (r *rawEntry) Object() *types.Object {
	return r.obj
}

func (r *rawEntry) Metadata() *types.Metadata {
	return &r.obj.Metadata
}

func (r *rawEntry) ExtendData() *types.ExtendData {
	return &r.obj.ExtendData
}

func (r *rawEntry) ExtendField() types.ExtendFields {
	r.obj.L.Lock()
	result := r.obj.ExtendFields
	r.obj.L.Unlock()
	return result
}

func (r *rawEntry) GetExtendField(fKey string) *string {
	r.obj.L.Lock()
	fVal, ok := r.obj.ExtendFields.Fields[fKey]
	r.obj.L.Unlock()
	if !ok {
		return nil
	}
	return &fVal
}

func (r *rawEntry) SetExtendField(fKey, fVal string) {
	r.obj.L.Lock()
	r.obj.ExtendFields.Fields[fKey] = fVal
	r.obj.L.Unlock()
}

func (r *rawEntry) RemoveExtendField(fKey string) error {
	r.obj.L.Lock()
	defer r.obj.L.Unlock()
	_, ok := r.obj.ExtendFields.Fields[fKey]
	if !ok {
		return types.ErrNotFound
	}
	delete(r.obj.ExtendFields.Fields, fKey)
	return nil
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

func initMirrorEntryObject(src, newParent *types.Object, attr EntryAttr) (*types.Object, error) {
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
	obj.Metadata.Inode = src.Inode
	obj.Metadata.Namespace = src.Namespace
	obj.RefID = src.ID
	return obj, nil
}
