package storage

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
)

const (
	LocalStorage         = "local"
	defaultLocalDirMode  = 0755
	defaultLocalFileMode = 0644
)

type local struct {
	dir    string
	logger *zap.SugaredLogger
}

var _ Storage = &local{}

func (l *local) ID() string {
	return LocalStorage
}

func (l *local) Get(ctx context.Context, key string, idx, offset int64) (io.ReadCloser, error) {
	file, err := l.openLocalFile(l.key2LocalPath(key, idx), os.O_RDWR)
	if err != nil {
		return nil, err
	}
	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		l.logger.Errorw("seek file failed", "key", key, "offset", offset, "err", err.Error())
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func (l *local) Put(ctx context.Context, key string, idx, offset int64, in io.Reader) error {
	file, err := l.openLocalFile(l.key2LocalPath(key, idx), os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		l.logger.Errorw("seek file failed", "key", key, "offset", offset, "err", err.Error())
		_ = file.Close()
		return err
	}
	_, err = io.Copy(file, in)
	return err
}

func (l *local) Delete(ctx context.Context, key string) error {
	p := path.Join(l.dir, key)
	_, err := os.Stat(p)
	if err != nil && !os.IsNotExist(err) {
		l.logger.Errorw("delete file failed", "key", key, "err", err.Error())
		return err
	}

	return os.RemoveAll(p)
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
		return nil, types.ErrNotFound
	}

	if info != nil && info.IsDir() {
		return nil, types.ErrIsGroup
	}

	f, err := os.OpenFile(path, flag, defaultLocalFileMode)
	if err != nil {
		l.logger.Errorw("open file failed", "path", path, "err", err.Error())
	}
	return f, err
}

func (l *local) key2LocalPath(key string, idx int64) string {
	dataPath := path.Join(l.dir, key)
	if _, err := os.Stat(dataPath); err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(dataPath, defaultLocalDirMode)
			if err != nil {
				l.logger.Errorw("data path mkdir failed", "path", dataPath)
			}
		}
	}
	return path.Join(l.dir, fmt.Sprintf("%s/%d", key, idx))
}

func newLocalStorage(dir string) Storage {
	return &local{
		dir:    dir,
		logger: logger.NewLogger("localStorage"),
	}
}
