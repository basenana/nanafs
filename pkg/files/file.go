package files

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
	"sync"
)

type File struct {
	*types.Object

	pageCache *pageNode

	attr Attr
	mux  sync.Mutex
}

func (f *File) Write(ctx context.Context, data []byte, offset int64) (n int64, err error) {
	if !f.attr.Write {
		return 0, types.ErrUnsupported
	}

	f.mux.Lock()
	defer f.mux.Unlock()

	pageStart := offset
	for {
		idx, pos := computePageIndex(pageStart)
		pageEnd := pageSize * (idx + 1)
		if pageEnd > int64(len(data)) {
			pageEnd = int64(len(data))
		}

		_ = insertPage(f.pageCache, idx, pos, data[pageStart:pageEnd])
		n += int64(len(data[pageStart:pageEnd]))

		pageStart = pageEnd

		if n == int64(len(data)) {
			break
		}
	}

	if offset+n > f.Object.Size {
		f.Object.Size = offset + n
	}
	return
}

func (f *File) Read(ctx context.Context, data []byte, offset int64) (n int, err error) {
	if !f.attr.Read {
		return 0, types.ErrUnsupported
	}

	var (
		pageStart = offset
		page      *pageNode
	)
	f.mux.Lock()
	for {
		idx, pos := computePageIndex(pageStart)
		pageEnd := pageSize * (idx + 1)
		if pageEnd > int64(len(data)) {
			pageEnd = int64(len(data))
		}
		page = findPage(f.pageCache, idx)

		n += copy(data[n:], page.date[pos:])
		if n == len(data) {
			break
		}

		pageStart = pageEnd
	}
	f.mux.Unlock()
	return
}

func (f *File) Fsync(ctx context.Context) error {
	if !f.attr.Write {
		return types.ErrUnsupported
	}
	f.mux.Lock()
	defer f.mux.Unlock()
	return nil
}

func (f *File) Flush(ctx context.Context) (err error) {
	err = commitDirtyPage(f.pageCache)
	return
}

func (f *File) Close(ctx context.Context) (err error) {
	return
}

type Attr struct {
	Read   bool
	Write  bool
	Create bool
}

func Open(ctx context.Context, obj *types.Object, attr Attr) (*File, error) {
	file := &File{
		Object: obj,
		attr:   attr,
	}
	return file, nil
}
