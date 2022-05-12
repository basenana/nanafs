// +build linux

package fs

import (
	"github.com/basenana/nanafs/pkg/dentry"
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

func nanaNode2Stat(node *NanaNode) *syscall.Stat_t {
	aTime, _ := unix.TimeToTimespec(node.obj.AccessAt)
	mTime, _ := unix.TimeToTimespec(node.obj.ModifiedAt)
	cTime, _ := unix.TimeToTimespec(node.obj.ChangedAt)

	mode := modeFromFileKind(node.obj.Kind)

	accMod := dentry.Access2Mode(node.obj.Access)
	mode |= accMod

	return &syscall.Stat_t{
		Size:    node.obj.Size,
		Blocks:  node.obj.Size/fileBlockSize + 1,
		Blksize: fileBlockSize,
		Atim:    syscall.Timespec{Sec: aTime.Sec, Nsec: aTime.Nsec},
		Mtim:    syscall.Timespec{Sec: mTime.Sec, Nsec: mTime.Nsec},
		Ctim:    syscall.Timespec{Sec: cTime.Sec, Nsec: cTime.Nsec},
		Mode:    mode,
		Ino:     node.obj.Inode,
		Nlink:   uint64(node.obj.RefCount),
		Uid:     uint32(node.obj.Access.UID),
		Gid:     uint32(node.obj.Access.GID),
		Rdev:    uint64(node.obj.Dev),
	}
}

func updateOsSideFields(attr *fuse.SetAttrIn, node *NanaNode) {}

func updateAttrOut(st *syscall.Stat_t, out *fuse.Attr) {
	out.FromStat(st)
}
