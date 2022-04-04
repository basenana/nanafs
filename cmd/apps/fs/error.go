package fs

import (
	"github.com/basenana/nanafs/pkg/object"
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
	case object.ErrNotFound:
		return syscall.ENOENT
	case object.ErrNoGroup:
		return syscall.Errno(20)
	case object.ErrIsGroup:
		return syscall.EISDIR

	}
	return syscall.EIO
}
