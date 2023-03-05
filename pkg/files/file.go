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

package files

import (
	"context"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"sync"
)

type File interface {
	GetObject() *types.Object
	GetAttr() Attr
	Write(ctx context.Context, data []byte, offset int64) (n int64, err error)
	Read(ctx context.Context, data []byte, offset int64) (int, error)
	Fsync(ctx context.Context) error
	Flush(ctx context.Context) (err error)
	Close(ctx context.Context) (err error)
}

type file struct {
	*types.Object

	dataChain chain

	attr Attr
	mux  sync.Mutex
}

func (f *file) GetObject() *types.Object {
	return f.Object
}

func (f *file) GetAttr() Attr {
	return f.attr
}

func (f *file) Write(ctx context.Context, data []byte, offset int64) (n int64, err error) {
	ctx, endF := utils.TraceTask(ctx, "file.write")
	defer endF()
	if !f.attr.Write {
		return 0, types.ErrUnsupported
	}

	var (
		shift     = offset
		leftSize  = int64(len(data))
		onceWrite int
	)
	f.mux.Lock()
	for {
		index, pos := computeChunkIndex(offset, fileChunkSize)

		chunkEnd := (index+1)*fileChunkSize - shift
		if chunkEnd > int64(len(data)) {
			chunkEnd = int64(len(data))
		}

		onceWrite, err = f.dataChain.writeAt(ctx, index, pos, data[n:chunkEnd])
		if err != nil {
			return
		}
		n += int64(onceWrite)
		if n == leftSize {
			break
		}
		offset += n
	}
	defer f.mux.Unlock()

	if offset+n > f.Object.Size {
		f.Object.Size = offset + n
	}
	return
}

func (f *file) Read(ctx context.Context, data []byte, offset int64) (n int, err error) {
	ctx, endF := utils.TraceTask(ctx, "file.read")
	defer endF()
	if !f.attr.Read {
		return 0, types.ErrUnsupported
	}

	var (
		shift    = offset
		onceRead int
	)

	leftSize := len(data)
	f.mux.Lock()
	for {
		if offset >= f.Size {
			break
		}

		index, pos := computeChunkIndex(offset, fileChunkSize)
		chunkEnd := (index+1)*fileChunkSize - shift
		if chunkEnd > int64(len(data)) {
			chunkEnd = int64(len(data))
		}
		if chunkEnd-int64(n) > f.Size-offset {
			chunkEnd = int64(n) + (f.Size - offset)
		}

		onceRead, err = f.dataChain.readAt(ctx, index, pos, data[n:chunkEnd])
		if err != nil {
			return
		}
		n += onceRead
		if n == leftSize {
			break
		}
		offset += int64(n)
	}
	f.mux.Unlock()
	return
}

func (f *file) Fsync(ctx context.Context) (err error) {
	defer utils.TraceRegion(ctx, "file.fsync")()
	if !f.attr.Write {
		return types.ErrUnsupported
	}
	f.mux.Lock()
	defer f.mux.Unlock()
	return nil
}

func (f *file) Flush(ctx context.Context) (err error) {
	defer utils.TraceRegion(ctx, "file.flush")()
	return
}

func (f *file) Close(ctx context.Context) (err error) {
	defer utils.TraceRegion(ctx, "file.close")()
	return f.dataChain.close(ctx)
}

type Attr struct {
	Read   bool
	Write  bool
	Create bool
	Trunc  bool
	Direct bool
	Meta   storage.Meta
}

func Open(ctx context.Context, obj *types.Object, attr Attr) (File, error) {
	defer utils.TraceRegion(ctx, "file.open")()
	if attr.Trunc {
		obj.Size = 0
	}

	switch obj.Kind {
	case types.SymLinkKind:
		return openSymlink(obj, attr)
	}

	f := &file{
		Object:    obj,
		dataChain: factory.build(obj, attr),
		attr:      attr,
	}
	return f, nil
}
