package restfs

import (
	"context"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	"io"
	"strings"
)

type file struct {
	f      files.File
	offset int64
}

func (f *file) Read(p []byte) (n int, err error) {
	n, err = f.f.Read(context.Background(), p, f.offset)
	f.offset += int64(n)
	if f.offset == f.f.GetObject().Size {
		err = io.EOF
	}
	return
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		f.offset = f.f.GetObject().Size + int64(whence)
	}
	return f.offset, nil
}

func pathEntries(path string) []string {
	path = strings.Trim(path, "/fs/")
	return strings.Split(path, "/")
}

func defaultAccess() []types.Permission {
	return []types.Permission{
		types.PermOwnerRead,
		types.PermOwnerWrite,
		types.PermGroupRead,
		types.PermGroupWrite,
		types.PermOthersRead,
	}
}
