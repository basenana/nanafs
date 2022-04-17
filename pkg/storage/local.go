package storage

import (
	"bufio"
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"io"
	"os"
	"path"
)

const (
	localStorageID   = "local"
	defaultLocalMode = 0755
)

type local struct {
	dir string
}

var _ Storage = &local{}

func (l *local) ID() string {
	return localStorageID
}

func (l *local) Get(ctx context.Context, key string, idx int64) (io.ReadCloser, error) {
	file, err := l.openLocalFile(l.key2LocalPath(key, idx), os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (l *local) Put(ctx context.Context, key string, in io.Reader, idx int64) error {
	file, err := l.openLocalFile(l.key2LocalPath(key, idx), os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	_, err = io.Copy(writer, in)
	return err
}

func (l *local) Delete(ctx context.Context, key string, idx int64) error {
	_, err := os.Stat(l.key2LocalPath(key, idx))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Remove(l.key2LocalPath(key, idx))
}

func (l *local) Head(ctx context.Context, key string, idx int64) (Info, error) {
	info, err := os.Stat(l.key2LocalPath(key, idx))
	if err != nil && !os.IsNotExist(err) {
		return Info{}, err
	}
	if os.IsNotExist(err) {
		return Info{}, types.ErrNotFound
	}
	return Info{
		Key:  info.Name(),
		Size: info.Size(),
	}, nil
}

func (l *local) openLocalFile(path string, flag int) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if os.IsNotExist(err) && flag&os.O_CREATE == 0 {
		return nil, err
	}

	if info.IsDir() {
		return nil, types.ErrIsGroup
	}

	return os.OpenFile(path, flag, defaultLocalMode)
}

func (l *local) key2LocalPath(key string, idx int64) string {
	return path.Join(l.dir, fmt.Sprintf("%s/%d", key, idx))
}

func newLocalStorage(dir string) Storage {
	return &local{
		dir: dir,
	}
}
