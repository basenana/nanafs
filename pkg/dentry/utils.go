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

const (
	RootEntryID   = 1
	RootEntryName = "root"
)

func initRootEntry() *types.Metadata {
	acc := &types.Access{
		Permissions: []types.Permission{
			types.PermOwnerRead,
			types.PermOwnerWrite,
			types.PermOwnerExec,
			types.PermGroupRead,
			types.PermGroupWrite,
			types.PermOthersRead,
		},
	}
	root, _ := types.InitNewEntry(nil, types.EntryAttr{Name: RootEntryName, Kind: types.GroupKind, Access: acc})
	root.ID = RootEntryID
	root.ParentID = root.ID
	return root
}

func initMirrorEntry(src, newParent *types.Metadata, attr types.EntryAttr) (*types.Metadata, error) {
	result, err := types.InitNewEntry(newParent, attr)
	if err != nil {
		return nil, err
	}

	result.Kind = src.Kind
	result.Namespace = src.Namespace
	result.RefID = src.ID
	return result, nil
}
