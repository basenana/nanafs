// +build darwin

package fs

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"golang.org/x/sys/unix"
	"syscall"
)

func fsMountOptions(displayName string, ops []string) []string {
	options := []string{fmt.Sprintf("volname=%s", displayName)}
	if ops != nil {
		options = append(options, ops...)
	}
	return options
}

func nanaNode2Stat(node *NanaNode) *syscall.Stat_t {
	aTime, _ := unix.TimeToTimespec(node.obj.AccessAt)
	mTime, _ := unix.TimeToTimespec(node.obj.ModifiedAt)
	cTime, _ := unix.TimeToTimespec(node.obj.ChangedAt)

	var mode uint16
	switch node.obj.Kind {
	case types.GroupKind:
		mode |= syscall.S_IFDIR
	default:
		mode |= syscall.S_IFREG
	}

	accMod := dentry.Access2Mode(node.obj.Access)
	mode |= uint16(accMod)

	return &syscall.Stat_t{
		Size:      node.obj.Size,
		Blocks:    node.obj.Size/fileBlockSize + 1,
		Blksize:   fileBlockSize,
		Atimespec: syscall.Timespec{Sec: aTime.Sec, Nsec: aTime.Nsec},
		Mtimespec: syscall.Timespec{Sec: mTime.Sec, Nsec: mTime.Nsec},
		Ctimespec: syscall.Timespec{Sec: cTime.Sec, Nsec: cTime.Nsec},
		Mode:      mode,
		Ino:       node.obj.Inode,
		Nlink:     0,
		Uid:       0,
		Gid:       0,
		Rdev:      0,
	}
}
