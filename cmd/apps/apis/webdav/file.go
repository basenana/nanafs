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
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
	"golang.org/x/net/webdav"
	"io/fs"
)

type File struct {
	core.File
}

func (f *File) Stat() (fs.FileInfo, error) {
	return f.File.Stat()
}

func (f *File) Readdir(count int) ([]fs.FileInfo, error) {

	var result []fs.FileInfo
	infos, err := f.File.Readdir(count)
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		result = append(result, info)
	}

	return result, nil
}

func (f *File) Close() error {
	return f.File.Close()
}

func openFile(ctx context.Context, entry *types.Entry, fs *core.FileSystem, attr types.OpenAttr) (webdav.File, error) {
	var (
		f   core.File
		err error
	)

	if entry.IsGroup {
		f, err = fs.OpenDir(ctx, entry.ID)
		if err != nil {
			return nil, err
		}
	} else {
		f, err = fs.Open(ctx, entry.ID, attr)
		if err != nil {
			return nil, err
		}
	}

	return &File{
		File: f,
	}, nil
}
