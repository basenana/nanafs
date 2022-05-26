package dentry

import (
	"github.com/basenana/nanafs/pkg/types"
)

const (
	RootObjectID = "root"
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
	root, _ := types.InitNewObject(nil, types.ObjectAttr{Name: RootObjectID, Kind: types.GroupKind, Access: acc})
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
	return !obj.IsGroup() && obj.RefID != "" && obj.RefID != obj.ID
}
