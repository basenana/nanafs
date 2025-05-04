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
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
	"io"
	"io/fs"
)

type File interface {
	io.ReadWriteCloser
	io.Seeker
	Readdir(count int) ([]fs.FileInfo, error)
	Stat() (fs.FileInfo, error)
}

type file struct {
	f        core.File
	entry    *types.Entry
	children []*types.Entry
}

func (f file) Read(p []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (f file) Write(p []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (f file) Close() error {
	//TODO implement me
	panic("implement me")
}

func (f file) Seek(offset int64, whence int) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (f file) Readdir(count int) ([]fs.FileInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (f file) Stat() (fs.FileInfo, error) {
	//TODO implement me
	panic("implement me")
}
