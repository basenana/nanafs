package workflow

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestWorkflow(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Workflow Suite")
}
