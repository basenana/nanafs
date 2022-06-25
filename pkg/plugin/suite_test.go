package plugin

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func NewMockStorage() storage.Storage {
	s, _ := storage.NewStorage("memory", config.Storage{})
	return s
}

func NewMockMeta() storage.MetaStore {
	m, _ := storage.NewMetaStorage("memory", config.Meta{})
	root := dentry.InitRootObject()
	_ = m.SaveObject(context.TODO(), nil, root)
	return m
}

func TestPlugin(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plugin Suite")
}
