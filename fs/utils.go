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

package fs

import (
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/jinzhu/copier"
)

func initRootEntry() *types.Entry {
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
	root, _ := types.InitNewEntry(nil, types.EntryAttr{Name: dentry.RootEntryName, Kind: types.GroupKind, Access: acc})
	root.ID = dentry.RootEntryID
	root.ParentID = root.ID
	root.Namespace = types.DefaultNamespace
	return root
}

func initNamespaceRootEntry(root *types.Entry, ns string) *types.Entry {
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
	nsRoot, _ := types.InitNewEntry(root, types.EntryAttr{Name: ns, Kind: types.GroupKind, Access: acc})
	nsRoot.Namespace = ns
	nsRoot.ParentID = root.ID
	return nsRoot
}

func entryInfo(entry *types.Entry) (*EntryInfo, error) {
	result := &EntryInfo{}
	return result, copier.Copy(result, entry)
}
