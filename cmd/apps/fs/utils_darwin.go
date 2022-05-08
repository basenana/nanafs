// +build darwin

package fs

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/hanwen/go-fuse/v2/fuse"
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

	mode := modeFromFileKind(node.obj.Kind)
	accMod := dentry.Access2Mode(node.obj.Access)
	mode |= accMod

	return &syscall.Stat_t{
		Size:      node.obj.Size,
		Blocks:    node.obj.Size/fileBlockSize + 1,
		Blksize:   fileBlockSize,
		Atimespec: syscall.Timespec{Sec: aTime.Sec, Nsec: aTime.Nsec},
		Mtimespec: syscall.Timespec{Sec: mTime.Sec, Nsec: mTime.Nsec},
		Ctimespec: syscall.Timespec{Sec: cTime.Sec, Nsec: cTime.Nsec},
		Mode:      uint16(mode),
		Ino:       node.obj.Inode,
		Nlink:     0,
		Uid:       uint32(node.obj.Access.UID),
		Gid:       uint32(node.obj.Access.GID),
		Rdev:      0,
	}
}

func updateOsSideFields(attr *fuse.SetAttrIn, node *NanaNode) {}

func updateAttrOut(st *syscall.Stat_t, out *fuse.Attr) {
	out.FromStat(st)

	// macos
	out.Crtime_ = uint64(st.Ctimespec.Sec)
	out.Crtimensec_ = uint32(st.Ctimespec.Nsec)
}
