package files

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	fileChunk1 []byte
	fileChunk2 []byte
	fileChunk3 []byte
	fileChunk4 []byte
)

func resetFileChunk() {
	fileChunk1 = make([]byte, fileChunkSize)
	fileChunk2 = make([]byte, fileChunkSize)
	fileChunk3 = make([]byte, fileChunkSize)
	fileChunk4 = make([]byte, fileChunkSize)
	copy(fileChunk1, []byte("testdata-1"))
	copy(fileChunk2, []byte("          "))
	copy(fileChunk3, []byte("testdata-3"))
	copy(fileChunk4, []byte(""))
}

func NewMockStorage() storage.Storage {
	s, _ := storage.NewStorage("memory", config.Storage{})
	InitLocalCache(config.Config{}, s)
	return s
}

func newMockObject(key string) *types.Object {
	meta := types.NewMetadata(key, types.RawKind)
	meta.ID = key
	return &types.Object{
		Metadata: meta,
	}
}

func TestFile(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "File Suite")
}
