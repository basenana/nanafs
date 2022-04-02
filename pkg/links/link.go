package links

import "github.com/basenana/nanafs/pkg/object"

type Link struct {
	object.Object
	Refs     []object.Object
	Incoming []object.Object
}
