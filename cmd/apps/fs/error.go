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
	case types.ErrNoGroup:
		return syscall.Errno(20)
	case types.ErrIsGroup:
		return syscall.EISDIR
	}
	return syscall.EIO
}
