package group

import (
	"github.com/basenana/nanafs/pkg/types"
)

type Group struct {
	types.Object
	Rule Rule
}
