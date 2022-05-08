package fs

import (
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
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

	updateOsSideFields(attr, node)
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

func fileKindFromMode(mode uint32) types.Kind {
	switch mode & syscall.S_IFMT {
	case syscall.S_IFREG:
		return types.RawKind
	case syscall.S_IFDIR:
		return types.GroupKind
	case syscall.S_IFLNK:
		return types.SymLinkKind
	case syscall.S_IFIFO:
		return types.FIFOKind
	case syscall.S_IFSOCK:
		return types.SocketKind
	case syscall.S_IFBLK:
		return types.BlkDevKind
	case syscall.S_IFCHR:
		return types.CharDevKind
	default:
		return types.RawKind
	}
}

func modeFromFileKind(kind types.Kind) uint32 {
	switch kind {
	case types.RawKind:
		return syscall.S_IFREG
	case types.GroupKind:
		return syscall.S_IFDIR
	case types.SymLinkKind:
		return syscall.S_IFLNK
	case types.FIFOKind:
		return syscall.S_IFIFO
	case types.SocketKind:
		return syscall.S_IFSOCK
	case types.BlkDevKind:
		return syscall.S_IFBLK
	case types.CharDevKind:
		return syscall.S_IFCHR
	default:
		return syscall.S_IFREG
	}
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
