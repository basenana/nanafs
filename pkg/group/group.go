package group

import "github.com/basenana/nanafs/pkg/object"

type Group struct {
	object.Object
	Rule Rule
}
