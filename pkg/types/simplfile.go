package types

import (
	"go/types"
	"io"
)

const (
	MirrorSourceLabelKey = "mirror.basenana.org/source"
)

type SimpleFile struct {
	Name    string
	IsGroup bool
	Source  string
	Open    func() (io.ReadWriteCloser, error)
}

func NewMirroredObject(sf SimpleFile) types.Object {
	return nil
}
