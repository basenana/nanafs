package fs

import (
	"github.com/basenana/nanafs/pkg/types"
)

const defaultMode = uint32(0777)

func IsAccess(access types.Access, mask uint32) error {
	return nil
}

func Access2Mode(access types.Access) uint32 {
	return defaultMode
}

func UpdateAccessWithMode(access *types.Access, mode uint32) {
	if mode == 0 {
		mode = defaultMode
	}
	return
}
