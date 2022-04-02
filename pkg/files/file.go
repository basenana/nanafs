package files

import (
	"context"
	"github.com/basenana/nanafs/pkg/object"
	"github.com/basenana/nanafs/pkg/storage"
	"sync"
)

type File struct {
	object.Object

	ref    int
	size   int64
	offset int64

	reader *reader
	writer *writer
	attr   Attr
	mux    sync.Mutex
}

func (f *File) Write(ctx context.Context, data []byte, offset int64) (n int64, err error) {
	if !f.attr.Write {
		return 0, object.ErrUnsupported
	}

	f.mux.Lock()
	defer f.mux.Unlock()
	if f.writer == nil {
		f.writer = &writer{f: f}
	}

	n, err = f.writer.write(ctx, data, offset)
	if err != nil {
		return
	}
	if offset+n > f.size {
		f.size = offset + n
	}
	return
}

func (f *File) Read(ctx context.Context, data []byte, offset int64) (int, error) {
	if !f.attr.Read {
		return 0, object.ErrUnsupported
	}

	f.mux.Lock()
	defer f.mux.Unlock()
	if f.reader == nil {
		f.reader = &reader{f: f}
	}

	return f.reader.read(ctx, data, offset)
}

func (f *File) Fsync(ctx context.Context) error {
	if !f.attr.Write {
		return object.ErrUnsupported
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
	Meta    storage.MetaStore
}

func Open(ctx context.Context, obj object.Object, attr Attr) (*File, error) {
	f, err := attr.Storage.Head(ctx, obj.GetObjectMeta().ID)
	if err != nil && err != object.ErrNotFound {
		return nil, err
	}

	if err == object.ErrNotFound && !attr.Create {
		return nil, err
	}

	file := &File{
		Object: obj,
		ref:    1,
		size:   f.Size,
		attr:   attr,
	}
	return file, nil
}

func Close(ctx context.Context, file *File) error {
	return file.Close(ctx)
}
