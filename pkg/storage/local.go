package storage

import (
	"bufio"
	"context"
	"github.com/basenana/nanafs/pkg/object"
	"io"
	"os"
	"path"
	"strings"
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

func (l *local) Get(ctx context.Context, key string, idx, off, limit int64) (io.ReadCloser, error) {
	file, err := l.openLocalFile(l.key2LocalPath(key), os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	if _, err = file.Seek(off, 0); err != nil {
		return nil, err
	}

	return file, nil
}

func (l *local) Put(ctx context.Context, key string, in io.Reader, idx, off int64) error {
	file, err := l.openLocalFile(l.key2LocalPath(key), os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err = file.Seek(off, 1); err != nil {
		return err
	}

	writer := bufio.NewWriter(file)
	_, err = io.Copy(writer, in)
	return err
}

func (l *local) Delete(ctx context.Context, key string) error {
	_, err := os.Stat(l.key2LocalPath(key))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Remove(l.key2LocalPath(key))
}

func (l *local) Head(ctx context.Context, key string) (Info, error) {
	info, err := os.Stat(l.key2LocalPath(key))
	if err != nil && !os.IsNotExist(err) {
		return Info{}, err
	}
	if os.IsNotExist(err) {
		return Info{}, object.ErrNotFound
	}
	return Info{
		Key:  info.Name(),
		Size: info.Size(),
	}, nil
}

func (l *local) Fsync(ctx context.Context, key string) error {
	//TODO implement me
	panic("implement me")
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
		return nil, object.ErrIsGroup
	}

	return os.OpenFile(path, flag, defaultLocalMode)
}

func (l *local) key2LocalPath(key string) string {
	prefix1 := string(key[0])
	parts := strings.Split(key, "-")
	return path.Join(l.dir, prefix1, parts[0])
}

func newLocalStorage(dir string) Storage {
	return &local{
		dir: dir,
	}
}
