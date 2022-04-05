package files

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/object"
	"github.com/basenana/nanafs/pkg/storage"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func NewMockStorage() storage.Storage {
	s, _ := storage.NewStorage("memory", config.Storage{})
	return s
}

type fileObject struct {
	meta object.Metadata
}

func (f *fileObject) GetObjectMeta() *object.Metadata {
	return &f.meta
}

func (f *fileObject) GetExtendData() *object.ExtendData {
	//TODO implement me
	panic("implement me")
}

func (f *fileObject) GetCustomColumn() *object.CustomColumn {
	//TODO implement me
	panic("implement me")
}

func TestFs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "File Suite")
}
