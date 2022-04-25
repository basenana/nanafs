package restfs

import (
	"github.com/basenana/nanafs/pkg/files"
	"strings"
)

type file struct {
	f files.File
}

func (f file) Read(p []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (f file) Seek(offset int64, whence int) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func pathEntries(path string) []string {
	path = strings.Trim(path, "/fs/")
	return strings.Split(path, "/")
}
