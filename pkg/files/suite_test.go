package files

import (
	"bytes"
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/object"
	"github.com/basenana/nanafs/pkg/storage"
	"io"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockStorage struct {
	storage map[string][]chunk
	mux     sync.Mutex
}

func (m *MockStorage) ID() string {
	return "mock-storage"
}

func (m *MockStorage) Get(ctx context.Context, key string, idx, off, limit int64) (io.ReadCloser, error) {
	chunks, err := m.getChunks(ctx, key)
	if err != nil {
		return nil, err
	}

	if idx > int64(len(chunks)) {
		return nil, fmt.Errorf("out of range")
	}

	c := chunks[idx]
	if int64(len(c.data)) < limit {
		limit = int64(len(c.data))
	}
	return dataReader{reader: bytes.NewReader(c.data[off:limit])}, nil
}

func (m *MockStorage) Put(ctx context.Context, key string, in io.Reader, idx, off int64) error {
	chunks, err := m.getChunks(ctx, key)
	if err != nil {
		chunks = make([]chunk, 0)
	}

	nowIdx := len(chunks)
	if idx+1 > int64(nowIdx) {
		sub := int(idx + 1 - int64(nowIdx))
		for sub > 0 {
			chunks = append(chunks, chunk{data: []byte{}})
			sub--
		}
	}

	var (
		c   = chunks[idx]
		buf = make([]byte, testChunkSize)
	)
	for {
		n, err := in.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		canRead := int(testChunkSize - off)
		if n > canRead {
			return fmt.Errorf("out of range")
		}

		c.data = append(c.data[:off], buf[:n]...)
	}
	chunks[idx] = c
	return m.saveChunks(ctx, key, chunks)
}

func (m *MockStorage) Delete(ctx context.Context, key string) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	_, ok := m.storage[key]
	if !ok {
		return object.ErrNotFound
	}
	delete(m.storage, key)
	return nil
}

func (m *MockStorage) Fsync(ctx context.Context, key string) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	return nil
}

func (m *MockStorage) Head(ctx context.Context, key string) (storage.Info, error) {
	result := storage.Info{Key: key}
	chunks, err := m.getChunks(ctx, key)
	if err != nil {
		return result, err
	}

	for _, c := range chunks {
		result.Size += int64(len(c.data))
	}
	return result, nil
}

func (m *MockStorage) getChunks(ctx context.Context, key string) ([]chunk, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	chunks, ok := m.storage[key]
	if !ok {
		return nil, object.ErrNotFound
	}
	return chunks, nil
}

func (m *MockStorage) saveChunks(ctx context.Context, key string, chunks []chunk) error {
	m.mux.Lock()
	m.storage[key] = chunks
	m.mux.Unlock()
	return nil
}

func NewMockStorage() storage.Storage {
	return &MockStorage{
		storage: map[string][]chunk{},
	}
}

type chunk struct {
	data   []byte
	offset int64
}

type fileObject struct {
	ID string
}

func (f fileObject) GetObjectMeta() object.Metadata {
	return object.Metadata{
		ID: f.ID,
	}
}

func (f fileObject) GetExtendData() object.ExtendData {
	//TODO implement me
	panic("implement me")
}

func (f fileObject) GetCustomColumn() object.CustomColumn {
	//TODO implement me
	panic("implement me")
}

func TestFs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "File Suite")
}
