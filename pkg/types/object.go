package types

type Object struct {
	*Metadata
}

func (e *Object) GetExtendData() *ExtendData {
	return &ExtendData{}
}

func (e *Object) GetCustomColumn() *CustomColumn {
	return &CustomColumn{}
}

func (e *Object) IsGroup() bool {
	return e.Kind == GroupKind
}

type ObjectAttr struct {
	Name string
	Mode uint32
	Kind Kind
}

func InitNewObject(parent *Object, attr ObjectAttr) (*Object, error) {
	newObj := &Object{
		Metadata: NewMetadata(attr.Name, attr.Kind),
	}
	if parent != nil {
		newObj.ParentID = parent.ID
	}
	return newObj, nil
}

const (
	RootObjectID = "root"
)

func InitRootObject() *Object {
	root, _ := InitNewObject(nil, ObjectAttr{Name: RootObjectID, Kind: GroupKind})
	root.ID = RootObjectID
	root.ParentID = root.ID
	return root
}
