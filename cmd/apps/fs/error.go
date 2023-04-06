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
	"github.com/basenana/nanafs/pkg/types"
	"syscall"
)

const (
	NoErr = syscall.Errno(0)
)

func Error2FuseSysError(err error) syscall.Errno {
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
	case types.ErrOutOfFS:
		return syscall.EXDEV
	case types.ErrUnsupported:
		return syscall.EBADF
	}
	return syscall.EIO
}
