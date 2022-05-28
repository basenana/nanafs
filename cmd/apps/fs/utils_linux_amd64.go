// +build linux amd64

package fs

import (
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/hanwen/go-fuse/v2/fuse"
	"golang.org/x/sys/unix"
	"syscall"
)

func fsMountOptions(displayName string, ops []string) []string {
	options := make([]string, 0)
	if ops != nil {
		options = append(options, ops...)
	}
	return options
}

func nanaNode2Stat(obj *types.Object) *syscall.Stat_t {
	aTime, _ := unix.TimeToTimespec(obj.AccessAt)
	mTime, _ := unix.TimeToTimespec(obj.ModifiedAt)
	cTime, _ := unix.TimeToTimespec(obj.ChangedAt)

	mode := modeFromFileKind(obj.Kind)

	accMod := dentry.Access2Mode(obj.Access)
	mode |= accMod

	return &syscall.Stat_t{
		Size:    obj.Size,
		Blocks:  obj.Size/fileBlockSize + 1,
		Blksize: fileBlockSize,
		Atim:    syscall.Timespec{Sec: aTime.Sec, Nsec: aTime.Nsec},
		Mtim:    syscall.Timespec{Sec: mTime.Sec, Nsec: mTime.Nsec},
		Ctim:    syscall.Timespec{Sec: cTime.Sec, Nsec: cTime.Nsec},
		Mode:    mode,
		Ino:     obj.Inode,
		Nlink:   uint64(obj.RefCount),
		Uid:     uint32(obj.Access.UID),
		Gid:     uint32(obj.Access.GID),
		Rdev:    uint64(obj.Dev),
	}
}

func updateAttrOut(st *syscall.Stat_t, out *fuse.Attr) {
	out.FromStat(st)
}
