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
	//swapped := (uint64(st.Dev) << 32) | (uint64(st.Dev) >> 32)
	//swappedRootDev := (dev << 32) | (dev >> 32)
	return fs.StableAttr{
		Mode: uint32(st.Mode),
		Gen:  1,
		Ino:  st.Ino,
	}
}

func updateNanaNodeWithAttr(attr *fuse.SetAttrIn, obj *types.Object, crtUid, crtGid int64, fileOpenAttr files.Attr) error {
	// do check
	if _, ok := attr.GetMode(); ok {
		if crtUid != 0 && crtUid != obj.Access.UID {
			return types.ErrNoPerm
		}
	}
	if _, ok := attr.GetUID(); ok {
		if crtUid != 0 && crtUid != obj.Access.UID {
			return types.ErrNoPerm
		}
	}
	if gid, ok := attr.GetGID(); ok {
		/*
			Only a privileged process (Linux: one with the CAP_CHOWN
			capability) may change the owner of a file.  The owner of a file
			may change the group of the file to any group of which that owner
			is a member.  A privileged process (Linux: with CAP_CHOWN) may
			change the group arbitrarily.
		*/
		if crtUid != 0 && crtUid != obj.Access.UID {
			return types.ErrNoPerm
		}
		if crtUid != 0 && int64(gid) != crtGid && !dentry.MatchUserGroup(crtUid, int64(gid)) {
			return types.ErrNoAccess
		}
	}
	if _, ok := attr.GetSize(); ok {
		if !fileOpenAttr.Create {
			if err := dentry.IsAccess(obj.Access, crtUid, crtGid, 0x2); err != nil {
				return err
			}
		}
	}

	// do update
	if mode, ok := attr.GetMode(); ok {
		if mode&syscall.S_ISUID > 0 && crtUid != 0 && crtUid != obj.Access.UID {
			mode ^= syscall.S_ISUID
		}
		if mode&syscall.S_ISGID > 0 && crtUid != 0 && crtGid != obj.Access.GID {
			mode ^= syscall.S_ISGID
		}
		dentry.UpdateAccessWithMode(&obj.Access, mode)
	}

	ownerUpdated := false
	if uid, ok := attr.GetUID(); ok {
		ownerUpdated = true
		dentry.UpdateAccessWithOwnID(&obj.Access, int64(uid), obj.Access.GID)
	}
	if gid, ok := attr.GetGID(); ok {
		ownerUpdated = true
		dentry.UpdateAccessWithOwnID(&obj.Access, obj.Access.UID, int64(gid))
	}
	if ownerUpdated {
		/*
		   When the owner or group of an executable file is changed by an
		   unprivileged user, the S_ISUID and S_ISGID mode bits are cleared.
		   POSIX does not specify whether this also should happen when root
		   does the chown(); the Linux behavior depends on the kernel
		   version, and since Linux 2.2.13, root is treated like other
		   users.  In case of a non-group-executable file (i.e., one for
		   which the S_IXGRP bit is not set) the S_ISGID bit indicates
		   mandatory locking, and is not cleared by a chown().
		*/
		(&obj.Access).RemovePerm(types.PermSetUid)
		(&obj.Access).RemovePerm(types.PermSetGid)
	}

	if size, ok := attr.GetSize(); ok {
		obj.Size = int64(size)
	}

	if mtime, ok := attr.GetMTime(); ok {
		obj.ModifiedAt = mtime
	}
	if atime, ok := attr.GetATime(); ok {
		obj.AccessAt = atime
	}
	if ctime, ok := attr.GetCTime(); ok {
		obj.ChangedAt = ctime
	}

	return nil
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
