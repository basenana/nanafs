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
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"golang.org/x/net/webdav"
	"io"
	"io/fs"
	"time"
)

type File struct {
	file dentry.File
	off  int64
	size int64
}

func (f *File) Read(p []byte) (int, error) {
	cnt, err := f.file.ReadAt(context.TODO(), p, f.off)
	f.off += cnt
	return int(cnt), err
}

func (f *File) Write(p []byte) (n int, err error) {
	cnt, err := f.file.WriteAt(context.TODO(), p, f.off)
	f.off += cnt
	if f.off > f.size {
		f.size = f.off
	}
	return int(cnt), err
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.off = offset
	case io.SeekCurrent:
		f.off += offset
	case io.SeekEnd:
		f.off = f.file.Metadata().Size + offset
	}
	return f.off, nil
}

func (f *File) Stat() (fs.FileInfo, error) {
	info := Stat(f.file.Metadata())
	info.size = f.size
	return info, nil
}

func (f *File) Close() error {
	return f.file.Close(context.TODO())
}

func (f *File) Readdir(count int) ([]fs.FileInfo, error) {
	return nil, types.ErrNoGroup
}

type Dir struct {
	group dentry.Entry
}

func (d *Dir) Readdir(count int) ([]fs.FileInfo, error) {
	if !d.group.IsGroup() {
		return nil, types.ErrNoGroup
	}
	children, err := d.group.Group().ListChildren(context.TODO())
	if err != nil {
		return nil, err
	}
	infos := make([]fs.FileInfo, len(children))
	for i := range children {
		infos[i] = Stat(children[i].Metadata())
	}

	if count <= 0 {
		return infos, nil
	}
	if count > len(infos) {
		count = len(infos)
	}
	return infos[:count], nil
}

func (d *Dir) Stat() (fs.FileInfo, error) {
	return Stat(d.group.Metadata()), nil
}

func (d *Dir) Write(p []byte) (int, error) {
	return 0, types.ErrIsGroup
}

func (d *Dir) Read(p []byte) (int, error) {
	return 0, types.ErrIsGroup
}

func (d *Dir) Seek(offset int64, whence int) (int64, error) {
	return 0, types.ErrIsGroup
}

func (d *Dir) Close() error {
	return nil
}

func openFile(entry dentry.Entry) (webdav.File, error) {
	if entry.IsGroup() {
		return &Dir{group: entry}, nil
	}
	file, ok := entry.(dentry.File)
	if !ok {
		return nil, fmt.Errorf("not a opened file")
	}
	return &File{
		file: file,
		size: entry.Metadata().Size,
	}, nil
}

type Info struct {
	name  string
	size  int64
	mode  uint32
	mTime time.Time
	isDir bool
}

func (i Info) Name() string {
	return i.name
}

func (i Info) Size() int64 {
	return i.size
}

func (i Info) Mode() fs.FileMode {
	return fs.FileMode(i.mode)
}

func (i Info) ModTime() time.Time {
	return i.mTime
}

func (i Info) IsDir() bool {
	return i.isDir
}

func (i Info) Sys() any {
	return nil
}
