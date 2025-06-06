/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package core

import (
	"github.com/basenana/nanafs/pkg/types"
	"golang.org/x/sys/unix"
	"os/user"
	"strconv"
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
		types.PermSetUid:      unix.S_ISUID,
		types.PermSetGid:      unix.S_ISGID,
		types.PermSticky:      unix.S_ISVTX,
	}
)

func HasAllPermissions(access types.Access, callerUid, callerGid int64, permissions ...types.Permission) error {
	var mask uint32
	for _, p := range permissions {
		mask |= perm2Mode[p]
	}
	if mask == 0 {
		return nil
	}
	return IsAccess(access, callerUid, callerGid, mask)
}

func IsAccess(access types.Access, callerUid, callerGid int64, mask uint32) error {
	if callerUid == 0 {
		// root can do anything.
		return nil
	}
	mask = mask & 7
	if mask == 0 {
		return nil
	}

	perm := Access2Mode(access)

	// as owner
	if callerUid == access.UID {
		if perm&(mask<<6) == mask<<6 {
			return nil
		}
		return types.ErrNoAccess
	}

	if callerGid == access.GID || MatchUserGroup(callerUid, access.GID) {
		if perm&(mask<<3) == mask<<3 {
			return nil
		}
		return types.ErrNoAccess
	}

	if perm&mask == mask {
		return nil
	}

	return types.ErrNoAccess
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

func MatchUserGroup(callerUid, targetGid int64) bool {
	u, err := user.LookupId(strconv.Itoa(int(callerUid)))
	if err != nil {
		return false
	}
	gs, err := u.GroupIds()
	if err != nil {
		return false
	}

	fileGidStr := strconv.Itoa(int(targetGid))
	for _, gidStr := range gs {
		if gidStr == fileGidStr {
			return true
		}
	}
	return false
}
