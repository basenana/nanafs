package types

import "io"

type SimpleFile struct {
	Name    string
	IsGroup bool
	io.ReadWriteCloser
}

func NewMirroredObject(sf SimpleFile) {

}
