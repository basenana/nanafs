package dentry

import (
	"github.com/basenana/nanafs/pkg/types"
	"golang.org/x/sys/unix"
)

var (
	perm2Mode = map[types.Permission]uint32{
		types.PermOwnerRead:   unix.S_IRUSR,
		types.PermOwnerWrite:  unix.S_IWUSR,
		types.PermOwnerExec:   unix.S_IXUSR,
		types.PermGroupRead:   unix.S_IRGRP,
		types.PermGroupWrite:  unix.S_IWGRP,
		types.PermGroupExec:   unix.S_IXGRP,
		types.PermOthersRead:  unix.S_IROTH,
		types.PermOthersWrite: unix.S_IWOTH,
		types.PermOthersExec:  unix.S_IXOTH,
	}
)

func IsAccess(access types.Access, mask uint32) error {
	return nil
}

func Access2Mode(access types.Access) (mode uint32) {
	for _, perm := range access.Permissions {
		m := perm2Mode[perm]
		mode |= m
	}
	return
}

func UpdateAccessWithMode(access *types.Access, mode uint32) {
	var permissions []types.Permission
	for perm, m := range perm2Mode {
		if m&mode > 0 {
			permissions = append(permissions, perm)
		}
	}
	access.Permissions = permissions
}

func UpdateAccessWithOwnID(access *types.Access, uid, gid int64) {
	access.UID = uid
	access.GID = gid
}
