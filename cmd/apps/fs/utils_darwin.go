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

// +build darwin

package fs

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
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

func nanaNode2Stat(obj *types.Object) *syscall.Stat_t {
	aTime, _ := unix.TimeToTimespec(obj.AccessAt)
	mTime, _ := unix.TimeToTimespec(obj.ModifiedAt)
	cTime, _ := unix.TimeToTimespec(obj.ChangedAt)

	mode := modeFromFileKind(obj.Kind)
	accMod := dentry.Access2Mode(obj.Access)
	mode |= accMod

	return &syscall.Stat_t{
		Size:      obj.Size,
		Blocks:    obj.Size/fileBlockSize + 1,
		Blksize:   fileBlockSize,
		Atimespec: syscall.Timespec{Sec: aTime.Sec, Nsec: aTime.Nsec},
		Mtimespec: syscall.Timespec{Sec: mTime.Sec, Nsec: mTime.Nsec},
		Ctimespec: syscall.Timespec{Sec: cTime.Sec, Nsec: cTime.Nsec},
		Mode:      uint16(mode),
		Ino:       obj.Inode,
		Nlink:     uint16(obj.RefCount),
		Uid:       uint32(obj.Access.UID),
		Gid:       uint32(obj.Access.GID),
		Rdev:      int32(obj.Dev),
	}
}

func updateAttrOut(st *syscall.Stat_t, out *fuse.Attr) {
	out.FromStat(st)

	// macos
	out.Crtime_ = uint64(st.Ctimespec.Sec)
	out.Crtimensec_ = uint32(st.Ctimespec.Nsec)
}
