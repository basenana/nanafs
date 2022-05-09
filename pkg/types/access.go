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
