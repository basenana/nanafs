package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

const (
	LocalStorage         = "local"
	defaultLocalDirMode  = 0755
	defaultLocalFileMode = 0644
)

type Info struct {
	Key  string
	Size int64
}

type Storage interface {
	ID() string
	Get(ctx context.Context, key int64, idx, offset int64) (io.ReadCloser, error)
	Put(ctx context.Context, key int64, idx, offset int64, in io.Reader) error
	Delete(ctx context.Context, key int64) error
	Head(ctx context.Context, key int64, idx int64) (Info, error)
}

const (
	MemoryStorage = "memory"
)

func NewStorage(storageID string, cfg config.Storage) (Storage, error) {
	switch storageID {
	case LocalStorage:
		return newLocalStorage(cfg.LocalDir), nil
	case MemoryStorage:
		return newMemoryStorage(), nil
	default:
		return nil, fmt.Errorf("unknow storage id: %s", storageID)
	}
}

func newMemoryMetaStore() Meta {
	return &memoryMetaStore{objects: map[int64]*types.Object{}}
}

type memoryStorage struct {
	storage map[string]chunk
	mux     sync.Mutex
}

func (m *memoryStorage) ID() string {
	return MemoryStorage
}

func (m *memoryStorage) Get(ctx context.Context, key int64, idx, offset int64) (io.ReadCloser, error) {
	defer utils.TraceRegion(ctx, "memory.get")()
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		return nil, err
	}
	return utils.NewDateReader(bytes.NewReader(ck.data[offset:])), nil
}

func (m *memoryStorage) Put(ctx context.Context, key int64, idx, offset int64, in io.Reader) error {
	defer utils.TraceRegion(ctx, "memory.put")()
	cKey := m.chunkKey(key, idx)
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		ck = &chunk{data: make([]byte, 1<<22)}
	}

	var (
		n   int
		buf = make([]byte, 1024)
	)

	for {
		n, err = in.Read(buf)
		if err == io.EOF || n == 0 {
			break
		}
		offset += int64(copy(ck.data[offset:], buf[:n]))
	}

	return m.saveChunk(ctx, cKey, *ck)
}

func (m *memoryStorage) Delete(ctx context.Context, key int64) error {
	defer utils.TraceRegion(ctx, "memory.delete")()
	m.mux.Lock()
	defer m.mux.Unlock()
	for k := range m.storage {
		if strings.HasPrefix(k, strconv.FormatInt(key, 10)) {
			delete(m.storage, k)
		}
	}
	return nil
}

func (m *memoryStorage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	defer utils.TraceRegion(ctx, "memory.head")()
	result := Info{Key: strconv.FormatInt(key, 10)}
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		return result, err
	}

	result.Size = int64(len(ck.data))
	return result, nil
}

func (m *memoryStorage) chunkKey(key int64, idx int64) string {
	return fmt.Sprintf("%d_%d", key, idx)
}

func (m *memoryStorage) getChunk(ctx context.Context, key string) (*chunk, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	chunk, ok := m.storage[key]
	if !ok {
		return nil, types.ErrNotFound
	}
	return &chunk, nil
}

func (m *memoryStorage) saveChunk(ctx context.Context, key string, chunk chunk) error {
	m.mux.Lock()
	m.storage[key] = chunk
	m.mux.Unlock()
	return nil
}

func newMemoryStorage() Storage {
	return &memoryStorage{
		storage: map[string]chunk{},
	}
}

type chunk struct {
	data []byte
}

type local struct {
	dir    string
	logger *zap.SugaredLogger
}

var _ Storage = &local{}

func (l *local) ID() string {
	return LocalStorage
}

func (l *local) Get(ctx context.Context, key int64, idx, offset int64) (io.ReadCloser, error) {
	defer utils.TraceRegion(ctx, "local.get")()
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

func (l *local) Put(ctx context.Context, key int64, idx, offset int64, in io.Reader) error {
	defer utils.TraceRegion(ctx, "local.put")()
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

func (l *local) Delete(ctx context.Context, key int64) error {
	defer utils.TraceRegion(ctx, "local.delete")()
	p := path.Join(l.dir, fmt.Sprintf("%d", key))
	_, err := os.Stat(p)
	if err != nil && !os.IsNotExist(err) {
		l.logger.Errorw("delete file failed", "key", key, "err", err.Error())
		return err
	}

	return os.RemoveAll(p)
}

func (l *local) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	defer utils.TraceRegion(ctx, "local.head")()
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

func (l *local) key2LocalPath(key int64, idx int64) string {
	dataPath := path.Join(l.dir, fmt.Sprintf("%d", key))
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
