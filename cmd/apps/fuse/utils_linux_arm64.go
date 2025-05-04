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

package fuse

import (
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
	"golang.org/x/sys/unix"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	ENOAttr = syscall.ENODATA
	ENODATA = syscall.ENODATA
)

func fsMountOptions(displayName string, ops []string) []string {
	options := make([]string, 0)
	if ops != nil {
		options = append(options, ops...)
	}
	return options
}

func nanaNode2Stat(entry *types.Entry) *syscall.Stat_t {
	aTime, _ := unix.TimeToTimespec(entry.AccessAt)
	mTime, _ := unix.TimeToTimespec(entry.ModifiedAt)
	cTime, _ := unix.TimeToTimespec(entry.ChangedAt)

	mode := modeFromFileKind(entry.Kind)

	accMod := dentry.Access2Mode(entry.Access)
	mode |= accMod

	rdev := MountDev
	if entry.Dev != 0 {
		rdev = uint64(entry.Dev)
	}

	return &syscall.Stat_t{
		Size:    entry.Size,
		Blocks:  entry.Size/fileBlockSize + 1,
		Blksize: fileBlockSize,
		Atim:    syscall.Timespec{Sec: aTime.Sec, Nsec: aTime.Nsec},
		Mtim:    syscall.Timespec{Sec: mTime.Sec, Nsec: mTime.Nsec},
		Ctim:    syscall.Timespec{Sec: cTime.Sec, Nsec: cTime.Nsec},
		Mode:    mode,
		Ino:     uint64(entry.ID),
		Nlink:   uint32(entry.RefCount),
		Uid:     uint32(entry.Access.UID),
		Gid:     uint32(entry.Access.GID),
		Rdev:    rdev,
	}
}

func updateAttrOut(st *syscall.Stat_t, out *fuse.Attr) {
	out.FromStat(st)
}
