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

import "github.com/basenana/nanafs/pkg/types"

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

func patchChangeableMetadata(oldMd, newMd *types.Metadata) {
	oldMd.Name = newMd.Name
	oldMd.Aliases = newMd.Aliases
	oldMd.ParentID = newMd.ParentID
	oldMd.RefID = newMd.RefID
	oldMd.RefCount = newMd.RefCount
	oldMd.Size = newMd.Size
	oldMd.Dev = newMd.Dev
	oldMd.Storage = newMd.Storage
	oldMd.Namespace = newMd.Namespace
	oldMd.CreatedAt = newMd.CreatedAt
	oldMd.ChangedAt = newMd.ChangedAt
	oldMd.ModifiedAt = newMd.ModifiedAt
	oldMd.AccessAt = newMd.AccessAt
	oldMd.Access = newMd.Access
}
