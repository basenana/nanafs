package dentry

import (
	"github.com/basenana/nanafs/pkg/types"
)

type Link struct {
	types.Object
	Refs     []types.Object
	Incoming []types.Object
}
