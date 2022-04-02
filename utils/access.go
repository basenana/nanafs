package utils

import (
	"github.com/basenana/nanafs/pkg/object"
)

const defaultMode = uint32(0777)

func IsAccess(access object.Access, mask uint32) error {
	return nil
}

func Access2Mode(access object.Access) uint32 {
	return defaultMode
}

func UpdateAccessWithMode(access *object.Access, mode uint32) {
	if mode == 0 {
		mode = defaultMode
	}
	return
}
