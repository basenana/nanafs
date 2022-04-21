package files

import (
	"context"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"sync"
)

type File interface {
	GetObject() *types.Object
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

func (f *file) Write(ctx context.Context, data []byte, offset int64) (n int64, err error) {
	if !f.attr.Write {
		return 0, types.ErrUnsupported
	}

	leftSize := int64(len(data))
	f.mux.Lock()
	for {
		var n1 int
		index, pos := computeChunkIndex(offset, fileChunkSize)
		n1, err = f.dataChain.writeAt(ctx, index, pos, data[n:])
		if err != nil {
			return
		}
		n += int64(n1)
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
	if !f.attr.Read {
		return 0, types.ErrUnsupported
	}

	leftSize := len(data)
	f.mux.Lock()
	for {
		var n1 int
		index, pos := computeChunkIndex(offset, fileChunkSize)
		n1, err = f.dataChain.readAt(ctx, index, pos, data[n:])
		if err != nil {
			return
		}
		n += n1
		if n == leftSize {
			break
		}
		offset += int64(n)
	}
	f.mux.Unlock()
	return
}

func (f *file) Fsync(ctx context.Context) (err error) {
	if !f.attr.Write {
		return types.ErrUnsupported
	}
	f.mux.Lock()
	defer f.mux.Unlock()
	return nil
}

func (f *file) Flush(ctx context.Context) (err error) {
	return
}

func (f *file) Close(ctx context.Context) (err error) {
	return f.dataChain.close(ctx)
}

type Attr struct {
	Read   bool
	Write  bool
	Create bool
	Meta   storage.MetaStore
}

func openFile(ctx context.Context, obj *types.Object, attr Attr) (*file, error) {
	file := &file{
		Object:    obj,
		dataChain: factory.build(obj, attr),
		attr:      attr,
	}
	return file, nil
}

func Open(ctx context.Context, obj *types.Object, attr Attr) (File, error) {
	return openFile(ctx, obj, attr)
}
