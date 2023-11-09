/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package fs

import (
	"encoding/base64"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	fileBlockSize = 1 << 12 // 4k
	NoErr         = syscall.Errno(0)

	RenameNoreplace = 0x1
	RenameExchange  = 0x2
	RenameWhiteout  = 0x4
)

var (
	operationLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "fuse_operation_latency_seconds",
			Help:    "The latency of fuse operation.",
			Buckets: prometheus.ExponentialBuckets(0.00001, 5, 10),
		},
		[]string{"operation"},
	)
	unexpectedErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fuse_unexpected_errors",
			Help: "This count of fuse operation encountering unexpected errors",
		},
		[]string{"operation"},
	)

	MountDev uint64
)

func init() {
	prometheus.MustRegister(
		operationLatency,
		unexpectedErrorCounter,
	)
}

func Error2FuseSysError(operation string, err error) syscall.Errno {
	if err == nil {
		return NoErr
	}
	switch err {
	case types.ErrNotFound:
		return syscall.ENOENT
	case types.ErrIsExist:
		return syscall.EEXIST
	case types.ErrNoGroup:
		return syscall.Errno(20)
	case types.ErrNotEmpty:
		return syscall.ENOTEMPTY
	case types.ErrIsGroup:
		return syscall.EISDIR
	case types.ErrNoAccess:
		return syscall.EACCES
	case types.ErrNoPerm:
		return syscall.EPERM
	case types.ErrNameTooLong:
		return syscall.ENAMETOOLONG
	case types.ErrUnsupported:
		return syscall.EBADF
	}
	unexpectedErrorCounter.WithLabelValues(operation).Inc()
	return syscall.EIO
}

func idFromStat(st *syscall.Stat_t) fs.StableAttr {
	//swapped := (uint64(st.Dev) << 32) | (uint64(st.Dev) >> 32)
	//swappedRootDev := (dev << 32) | (dev >> 32)
	return fs.StableAttr{
		Mode: uint32(st.Mode),
		Gen:  1,
		Ino:  st.Ino,
	}
}

func updateNanaNodeWithAttr(attr *fuse.SetAttrIn, entry *types.Metadata, crtUid, crtGid int64, fileOpenAttr types.OpenAttr) error {
	// do check
	if _, ok := attr.GetMode(); ok {
		if crtUid != 0 && crtUid != entry.Access.UID {
			return types.ErrNoPerm
		}
	}
	if _, ok := attr.GetUID(); ok {
		if crtUid != 0 && crtUid != entry.Access.UID {
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
		if crtUid != 0 && crtUid != entry.Access.UID {
			return types.ErrNoPerm
		}
		if crtUid != 0 && int64(gid) != crtGid && !dentry.MatchUserGroup(crtUid, int64(gid)) {
			// types.ErrNoPerm or types.ErrNoAccess
			return types.ErrNoPerm
		}
	}
	if _, ok := attr.GetSize(); ok {
		if !fileOpenAttr.Create {
			if err := dentry.IsAccess(entry.Access, crtUid, crtGid, 0x2); err != nil {
				return err
			}
		}
	}

	// do update
	if mode, ok := attr.GetMode(); ok {
		if mode&syscall.S_ISUID > 0 && crtUid != 0 && crtUid != entry.Access.UID {
			mode ^= syscall.S_ISUID
		}
		if mode&syscall.S_ISGID > 0 && crtUid != 0 && crtGid != entry.Access.GID {
			mode ^= syscall.S_ISGID
		}
		dentry.UpdateAccessWithMode(&entry.Access, mode)
	}

	ownerUpdated := false
	if uid, ok := attr.GetUID(); ok {
		ownerUpdated = true
		dentry.UpdateAccessWithOwnID(&entry.Access, int64(uid), entry.Access.GID)
	}
	if gid, ok := attr.GetGID(); ok {
		ownerUpdated = true
		dentry.UpdateAccessWithOwnID(&entry.Access, entry.Access.UID, int64(gid))
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
		(&entry.Access).RemovePerm(types.PermSetUid)
		(&entry.Access).RemovePerm(types.PermSetGid)
	}

	if size, ok := attr.GetSize(); ok {
		entry.Size = int64(size)
	}

	if mtime, ok := attr.GetMTime(); ok {
		entry.ModifiedAt = mtime
	}
	if atime, ok := attr.GetATime(); ok {
		entry.AccessAt = atime
	}
	if ctime, ok := attr.GetCTime(); ok {
		entry.ChangedAt = ctime
	}

	return nil
}

func fsInfo2StatFs(info controller.Info, out *fuse.StatfsOut) {
	out.Blocks = info.MaxSize / fileBlockSize
	out.Bfree = (info.MaxSize - info.UsageSize) / fileBlockSize
	out.Bavail = out.Bfree
	out.Files = info.AvailInodes
	out.Ffree = info.AvailInodes - info.Objects
	out.Bsize = uint32(fileBlockSize)
	out.NameLen = 255
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
	case types.GroupKind, types.ExternalGroupKind:
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

func openFileAttr(flags uint32) types.OpenAttr {
	attr := types.OpenAttr{
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

func xattrRawData2Content(raw []byte) string {
	return base64.StdEncoding.EncodeToString(raw)
}

func xattrContent2RawData(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}

func logOperationLatency(operation string, startAt time.Time) {
	operationLatency.WithLabelValues(operation).Observe(time.Since(startAt).Seconds())
}
