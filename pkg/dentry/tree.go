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
	"github.com/basenana/nanafs/pkg/types"
)

const (
	RootObjectID   = -1
	RootObjectName = "root"
)

func InitRootObject() *types.Object {
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
	root, _ := types.InitNewObject(nil, types.ObjectAttr{Name: RootObjectName, Kind: types.GroupKind, Access: acc})
	root.ID = RootObjectID
	root.ParentID = root.ID
	return root
}

func CreateMirrorObject(src, newParent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	obj, err := types.InitNewObject(newParent, attr)
	if err != nil {
		return nil, err
	}

	obj.Metadata.Kind = src.Kind
	obj.Metadata.Inode = src.Inode
	obj.Metadata.Namespace = src.Namespace
	obj.RefID = src.ID
	return obj, nil
}

func IsMirrorObject(obj *types.Object) bool {
	return !obj.IsGroup() && obj.RefID != 0 && obj.RefID != obj.ID
}
