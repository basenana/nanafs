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

func NewMockStorage() storage.Storage {
	s, _ := storage.NewStorage("memory", config.Storage{})
	return s
}

func newMockObject(key string) types.Object {
	meta := types.NewMetadata(key, types.RawKind)
	meta.ID = key
	return types.Object{
		Metadata: meta,
	}
}

func TestFile(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "File Suite")
}
