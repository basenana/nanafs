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

package webdav

import (
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"io/fs"
	"os"
	"syscall"
)

func Stat(entry *types.Metadata) Info {
	return Info{
		name:  entry.Name,
		size:  entry.Size,
		mode:  modeFromFileKind(entry.Kind) | dentry.Access2Mode(entry.Access),
		mTime: entry.ModifiedAt,
		isDir: entry.IsGroup,
	}
}

func mode2EntryAttr(mode os.FileMode) types.EntryAttr {
	attr := types.EntryAttr{
		Kind:   fileKindFromMode(uint32(mode)),
		Access: &types.Access{},
	}
	dentry.UpdateAccessWithMode(attr.Access, uint32(mode))
	return attr
}

func flag2EntryOpenAttr(flags int) types.OpenAttr {
	attr := types.OpenAttr{
		Read: true,
	}
	if flags&os.O_CREATE > 0 {
		attr.Create = true
	}
	if flags&os.O_TRUNC > 0 {
		attr.Trunc = true
	}
	if flags&os.O_RDWR > 0 || flags&os.O_WRONLY > 0 {
		attr.Write = true
	}
	return attr
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

func error2FsError(err error) error {
	if err == nil {
		return nil
	}
	switch err {
	case types.ErrNotFound:
		return fs.ErrNotExist
	case types.ErrIsExist:
		return fs.ErrExist
	case types.ErrNoAccess, types.ErrNoPerm:
		return fs.ErrPermission
	case types.ErrNameTooLong:
		return fs.ErrInvalid
	default:
		return err
	}
}
