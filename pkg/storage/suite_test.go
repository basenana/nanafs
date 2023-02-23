package storage

import (
	"github.com/basenana/nanafs/utils/logger"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var workdir string

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)

	logger.InitLogger()
	defer logger.Sync()

	var err error
	workdir, err = os.MkdirTemp(os.TempDir(), "nanafsunittest-")
	Expect(err).Should(BeNil())
	t.Logf("unit test workdir on: %s", workdir)

	RunSpecs(t, "Storage Suite")
}
