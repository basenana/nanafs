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

package types

type Permission string

const (
	PermOwnerRead   = "owner_read"
	PermOwnerWrite  = "owner_write"
	PermOwnerExec   = "owner_exec"
	PermGroupRead   = "group_read"
	PermGroupWrite  = "group_write"
	PermGroupExec   = "group_exec"
	PermOthersRead  = "others_read"
	PermOthersWrite = "others_write"
	PermOthersExec  = "others_exec"
	PermSetUid      = "set_uid"
	PermSetGid      = "set_gid"
	PermSticky      = "sticky"
)

type Access struct {
	ID          int64        `json:"id"`
	Permissions []Permission `json:"permissions,omitempty"`
	UID         int64        `json:"uid"`
	GID         int64        `json:"gid"`
}

func (a *Access) HasPerm(p Permission) bool {
	for _, perm := range a.Permissions {
		if perm == p {
			return true
		}
	}
	return false
}

func (a *Access) AddPerm(p Permission) {
	for _, old := range a.Permissions {
		if old == p {
			return
		}
	}
	a.Permissions = append(a.Permissions, p)
}

func (a *Access) RemovePerm(p Permission) {
	for i, old := range a.Permissions {
		if old != p {
			continue
		}
		a.Permissions = append(a.Permissions[0:i], a.Permissions[i+1:]...)
	}
}
