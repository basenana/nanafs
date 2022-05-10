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
	Permissions []Permission `json:"permissions,omitempty"`
	UID         int64        `json:"uid"`
	GID         int64        `json:"gid"`
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
