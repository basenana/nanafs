package fs

import (
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"os"
	"syscall"
)

const fileBlockSize = 1 << 12 // 4k

func idFromStat(dev uint64, st *syscall.Stat_t) fs.StableAttr {
	swapped := (uint64(st.Dev) << 32) | (uint64(st.Dev) >> 32)
	swappedRootDev := (dev << 32) | (dev >> 32)
	return fs.StableAttr{
		Mode: uint32(st.Mode),
		Gen:  1,
		Ino:  (swapped ^ swappedRootDev) ^ st.Ino,
	}
}

func updateNanaNodeWithAttr(attr *fuse.SetAttrIn, node *NanaNode) {
	if mode, ok := attr.GetMode(); ok {
		dentry.UpdateAccessWithMode(&node.obj.Access, mode)
	}
	if uid, ok := attr.GetUID(); ok {
		dentry.UpdateAccessWithOwnID(&node.obj.Access, int64(uid), node.obj.Access.GID)
	}
	if gid, ok := attr.GetGID(); ok {
		dentry.UpdateAccessWithOwnID(&node.obj.Access, node.obj.Access.UID, int64(gid))
	}
	if size, ok := attr.GetSize(); ok {
		node.obj.Size = int64(size)
	}

	if mtime, ok := attr.GetMTime(); ok {
		node.obj.ModifiedAt = mtime
	}
	if atime, ok := attr.GetATime(); ok {
		node.obj.AccessAt = atime
	}
	if ctime, ok := attr.GetCTime(); ok {
		node.obj.ChangedAt = ctime
	}
}

func fsInfo2StatFs(info controller.Info, out *fuse.StatfsOut) {
	var blockSize uint64 = 32768

	out.Blocks = info.MaxSize / blockSize
	out.Bfree = (info.MaxSize - info.UsageSize) / blockSize
	out.Bavail = out.Bfree
	out.Files = info.AvailInodes
	out.Ffree = info.AvailInodes - info.FileCount
	out.Bsize = uint32(blockSize)
	out.NameLen = 1024
}

func openFileAttr(flags uint32) files.Attr {
	attr := files.Attr{
		Read: true,
	}
	if int(flags)&os.O_CREATE > 0 {
		attr.Create = true
	}
	if int(flags)&os.O_TRUNC > 0 {
		attr.Trunc = true
	}
	if int(flags)&os.O_RDWR > 0 || int(flags)&os.O_WRONLY > 0 {
		attr.Write = true
	}
	return attr
}
