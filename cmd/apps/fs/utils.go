package fs

import (
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/object"
	"github.com/basenana/nanafs/utils"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"golang.org/x/sys/unix"
	"os"
	"syscall"
)

func idFromStat(dev uint64, st *syscall.Stat_t) fs.StableAttr {
	swapped := (uint64(st.Dev) << 32) | (uint64(st.Dev) >> 32)
	swappedRootDev := (dev << 32) | (dev >> 32)
	return fs.StableAttr{
		Mode: uint32(st.Mode),
		Gen:  1,
		// This should work well for traditional backing FSes,
		// not so much for other go-fuse FS-es
		Ino: (swapped ^ swappedRootDev) ^ st.Ino,
	}
}

func nanaNode2Stat(node *NanaNode) *syscall.Stat_t {
	aTime, _ := unix.TimeToTimespec(node.entry.AddedAt)
	mTime, _ := unix.TimeToTimespec(node.entry.ModifiedAt)
	cTime, _ := unix.TimeToTimespec(node.entry.CreatedAt)

	var mode uint16
	switch node.entry.Kind {
	case object.GroupKind:
		mode |= syscall.S_IFDIR
	default:
		mode |= syscall.S_IFREG
	}

	accMod := utils.Access2Mode(node.entry.Access)
	mode |= uint16(accMod)

	return &syscall.Stat_t{
		Size:      node.entry.Size,
		Blocks:    1,
		Atimespec: syscall.Timespec{Sec: aTime.Sec, Nsec: aTime.Nsec},
		Mtimespec: syscall.Timespec{Sec: mTime.Sec, Nsec: mTime.Nsec},
		Ctimespec: syscall.Timespec{Sec: cTime.Sec, Nsec: cTime.Nsec},
		Mode:      mode,
		Nlink:     0,
		Uid:       0,
		Gid:       0,
		Rdev:      0,
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
	attr := files.Attr{}
	if int(flags)|os.O_CREATE > 0 {
		attr.Create = true
	}
	if int(flags)|os.O_RDWR > 0 {
		attr.Write = true
		attr.Read = true
	} else {
		if int(flags)|os.O_RDONLY > 0 {
			attr.Read = true
		}
		if int(flags)|os.O_WRONLY > 0 {
			attr.Write = true
		}
	}
	return attr
}
