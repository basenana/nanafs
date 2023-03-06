//go:build linux || amd64

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

func nanaNode2Stat(entry dentry.Entry) *syscall.Stat_t {
	meta := entry.Metadata()
	aTime, _ := unix.TimeToTimespec(meta.AccessAt)
	mTime, _ := unix.TimeToTimespec(meta.ModifiedAt)
	cTime, _ := unix.TimeToTimespec(meta.ChangedAt)

	mode := modeFromFileKind(meta.Kind)

	accMod := dentry.Access2Mode(meta.Access)
	mode |= accMod

	return &syscall.Stat_t{
		Size:    meta.Size,
		Blocks:  meta.Size/fileBlockSize + 1,
		Blksize: fileBlockSize,
		Atim:    syscall.Timespec{Sec: aTime.Sec, Nsec: aTime.Nsec},
		Mtim:    syscall.Timespec{Sec: mTime.Sec, Nsec: mTime.Nsec},
		Ctim:    syscall.Timespec{Sec: cTime.Sec, Nsec: cTime.Nsec},
		Mode:    mode,
		Ino:     meta.Inode,
		Nlink:   uint64(meta.RefCount),
		Uid:     uint32(meta.Access.UID),
		Gid:     uint32(meta.Access.GID),
		Rdev:    uint64(meta.Dev),
	}
}

func updateAttrOut(st *syscall.Stat_t, out *fuse.Attr) {
	out.FromStat(st)
}
