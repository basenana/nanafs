package storage

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fs Suite")
}
