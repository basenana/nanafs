package files

import (
	"context"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"sync"
)

type File struct {
	*types.Object

	ref       int
	offset    int64
	chunkSize int64

	reader *reader
	writer *writer
	attr   Attr
	mux    sync.Mutex
}

func (f *File) Write(ctx context.Context, data []byte, offset int64) (n int64, err error) {
	if !f.attr.Write {
		return 0, types.ErrUnsupported
	}

	f.mux.Lock()
	defer f.mux.Unlock()
	if f.writer == nil {
		f.writer = initFileWriter(f)
	}

	n, err = f.writer.write(ctx, data, offset)
	if err != nil {
		return
	}
	if offset+n > f.Object.Size {
		f.Object.Size = offset + n
	}
	return
}

func (f *File) Read(ctx context.Context, data []byte, offset int64) (int, error) {
	if !f.attr.Read {
		return 0, types.ErrUnsupported
	}

	f.mux.Lock()
	defer f.mux.Unlock()
	if f.reader == nil {
		f.reader = initFileReader(f)
	}

	return f.reader.read(ctx, data, offset)
}

func (f *File) Fsync(ctx context.Context) error {
	if !f.attr.Write {
		return types.ErrUnsupported
	}
	if f.writer == nil {
		return nil
	}
	f.mux.Lock()
	defer f.mux.Unlock()
	return nil
}
func (f *File) Flush(ctx context.Context) (err error) {
	return
}

func (f *File) Close(ctx context.Context) (err error) {
	if f.reader != nil {
		err = f.reader.close(ctx)
	}
	if f.writer != nil {
		if err != nil {
			_ = f.writer.close(ctx)
		} else {
			err = f.writer.close(ctx)
		}
	}
	return
}

type Attr struct {
	Read    bool
	Write   bool
	Create  bool
	Storage storage.Storage
}

func Open(ctx context.Context, obj *types.Object, attr Attr) (*File, error) {
	f, err := attr.Storage.Head(ctx, obj.ID)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}

	if err == types.ErrNotFound && !attr.Create {
		return nil, err
	}

	obj.Size = f.Size

	file := &File{
		Object: obj,
		ref:    1,
		attr:   attr,

		chunkSize: defaultChunkSize,
	}
	return file, nil
}
