package plugin

type Type string

const (
	TypeMeta    Type = "meta"
	TypeSource  Type = "source"
	TypeProcess Type = "process"
	TypeMirror  Type = "mirror"
)
