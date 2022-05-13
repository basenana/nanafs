package fs

import (
	"github.com/basenana/nanafs/pkg/types"
	"syscall"
)

const (
	NoErr = syscall.Errno(0)
)

func Error2FuseSysError(err error) syscall.Errno {
	if err == nil {
		return NoErr
	}
	switch err {
	case types.ErrNotFound:
		return syscall.ENOENT
	case types.ErrIsExist:
		return syscall.EEXIST
	case types.ErrNoGroup:
		return syscall.Errno(20)
	case types.ErrNotEmpty:
		return syscall.ENOTEMPTY
	case types.ErrIsGroup:
		return syscall.EISDIR
	case types.ErrNoAccess:
		return syscall.EACCES
	case types.ErrNoPerm:
		return syscall.EPERM
	case types.ErrNameTooLong:
		return syscall.ENAMETOOLONG
	case types.ErrOutOfFS:
		return syscall.EXDEV
	}
	return syscall.EIO
}
