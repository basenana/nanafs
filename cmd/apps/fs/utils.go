package fs

import (
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"os"
	"syscall"
	"time"
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
	dentry.UpdateAccessWithMode(&node.obj.Access, attr.Mode)
	if attr.Atime != 0 {
		node.obj.AccessAt = time.Unix(int64(attr.Atime), int64(attr.Atimensec))
	}
	if attr.Ctime != 0 {
		node.obj.ChangedAt = time.Unix(int64(attr.Ctime), int64(attr.Ctimensec))
	}
	if attr.Mtime != 0 {
		node.obj.ModifiedAt = time.Unix(int64(attr.Mtime), int64(attr.Mtimensec))
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
