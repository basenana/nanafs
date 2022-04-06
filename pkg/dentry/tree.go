package dentry

import "github.com/basenana/nanafs/pkg/types"

const (
	RootObjectID = "root"
)

func InitRootObject() *types.Object {
	root, _ := types.InitNewObject(nil, types.ObjectAttr{Name: RootObjectID, Kind: types.GroupKind})
	root.ID = RootObjectID
	root.ParentID = root.ID
	return root
}

func CreateMirrorObject(src, newParent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	obj, err := types.InitNewObject(newParent, attr)
	if err != nil {
		return nil, err
	}

	obj.Metadata = src.Metadata
	obj.RefID = src.ID
	return obj, nil
}

func IsMirrorObject(obj *types.Object) bool {
	return obj.RefID != ""
}
