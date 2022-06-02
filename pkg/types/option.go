package types

type ObjectAttr struct {
	Name   string
	Dev    int64
	Kind   Kind
	Access Access
}

type DestroyObjectAttr struct {
	Uid int64
	Gid int64
}

type ChangeParentAttr struct {
	Uid      int64
	Gid      int64
	Replace  bool
	Exchange bool
}

type ChangeParentOption struct{}
