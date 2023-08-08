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
	"github.com/basenana/nanafs/cmd/apps/apis/pathmgr"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"golang.org/x/net/webdav"
	"io"
	"io/fs"
	"time"
)

type File struct {
	mgr   *pathmgr.PathManager
	entry *types.Metadata
	file  dentry.File
	attr  dentry.Attr
	off   int64
	size  int64
}

func (f *File) Read(p []byte) (n int, err error) {
	if err = f.open(); err != nil {
		return 0, err
	}
	var cnt int64
	cnt, err = f.file.ReadAt(context.TODO(), p, f.off)
	f.off += cnt
	return int(cnt), err
}

func (f *File) Write(p []byte) (n int, err error) {
	if err = f.open(); err != nil {
		return 0, err
	}
	var cnt int64
	cnt, err = f.file.WriteAt(context.TODO(), p, f.off)
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
		f.off = f.entry.Size + offset
	}
	return f.off, nil
}

func (f *File) Stat() (fs.FileInfo, error) {
	info := Stat(f.entry)
	info.size = f.size
	return info, nil
}

func (f *File) Close() error {
	if f.file == nil {
		return nil
	}
	log.Infow("close file", "entry", f.entry.ID, "name", f.entry.Name)
	return f.file.Close(context.TODO())
}

func (f *File) Readdir(count int) ([]fs.FileInfo, error) {
	return nil, types.ErrNoGroup
}

func (f *File) open() (err error) {
	if f.file == nil {
		f.file, err = f.mgr.Open(context.TODO(), f.entry.ID, f.attr)
		if err != nil {
			log.Errorw("open file error", "entry", f.entry.ID, "err", err)
			return err
		}
		log.Infow("open file", "entry", f.entry.ID, "name", f.entry.Name)
	}
	return nil
}

type Dir struct {
	path  string
	mgr   *pathmgr.PathManager
	group *types.Metadata
}

func (d *Dir) Readdir(count int) ([]fs.FileInfo, error) {
	if !types.IsGroup(d.group.Kind) {
		return nil, types.ErrNoGroup
	}
	children, err := d.mgr.ListEntry(context.TODO(), d.path)
	if err != nil {
		return nil, err
	}
	infos := make([]fs.FileInfo, len(children))
	for i := range children {
		infos[i] = Stat(children[i])
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
	return Stat(d.group), nil
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

func openFile(enPath string, entry *types.Metadata, mgr *pathmgr.PathManager, attr dentry.Attr) (webdav.File, error) {
	if types.IsGroup(entry.Kind) {
		return &Dir{path: enPath, mgr: mgr, group: entry}, nil
	}
	return &File{
		entry: entry,
		mgr:   mgr,
		attr:  attr,
		size:  entry.Size,
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
