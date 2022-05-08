package controller

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
)

func fileAccessWithFsOwner(access types.Access, owner *config.FsOwner) types.Access {
	if owner == nil {
		return access
	}

	if access.UID == 0 && owner.Uid != 0 {
		access.UID = owner.Uid
	}
	if access.GID == 0 && owner.Gid != 0 {
		access.GID = owner.Gid
	}
	return access
}
