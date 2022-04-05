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

func InitNewEntry(parent *Object, attr ObjectAttr) (*Object, error) {
	newEntry := &Object{
		Metadata: NewMetadata(attr.Name, attr.Kind),
	}
	if parent != nil {
		newEntry.ParentID = parent.ID
	}
	return newEntry, nil
}

const (
	RootEntryID = "root"
)

func InitRootEntry() *Object {
	root, _ := InitNewEntry(nil, ObjectAttr{Name: RootEntryID, Kind: GroupKind})
	root.ID = RootEntryID
	root.ParentID = root.ID
	return root
}
