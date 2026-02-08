/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package jobrun

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	tempDir   string
	memMeta   metastore.Meta
	testCore  core.Core
	namespace = types.DefaultNamespace
)

func TestJobrun(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jobrun Suite")
}

var _ = BeforeSuite(func() {
	var err error
	tempDir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-jobrun-")
	Expect(err).Should(BeNil())

	memMeta, err = metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())

	bootCfg := config.Bootstrap{
		CacheDir:  tempDir,
		CacheSize: 0,
		FS:        &config.FS{},
		Storages:  []config.Storage{{ID: storage.MemoryStorage, Type: storage.MemoryStorage}},
	}

	testCore, err = core.New(memMeta, bootCfg)
	Expect(err).Should(BeNil())

	err = testCore.CreateNamespace(context.TODO(), namespace)
	Expect(err).Should(BeNil())
})

var _ = AfterSuite(func() {
	if tempDir != "" {
		os.RemoveAll(tempDir)
	}
})

type mockResults struct {
	data map[string]any
}

func (m *mockResults) Set(key string, val any) error {
	m.data[key] = val
	return nil
}

func (m *mockResults) Data() map[string]any {
	return m.data
}

var _ Results = &mockResults{}

// newWorkflowJob creates a test WorkflowJob with basic configuration
func newWorkflowJob(name string) *types.WorkflowJob {
	return &types.WorkflowJob{
		Id:        fmt.Sprintf("test-job-%s", name),
		Namespace: namespace,
		Workflow:  "test-wf",
		Targets: types.WorkflowTarget{
			Entries: []string{"/test/file.txt"},
		},
		Nodes: []types.WorkflowJobNode{
			{
				WorkflowNode: types.WorkflowNode{
					Name: "step1",
					Type: "test",
					Next: "step2",
				},
			},
			{
				WorkflowNode: types.WorkflowNode{
					Name: "step2",
					Type: "test",
				},
			},
		},
	}
}

// newWorkflowJobNode creates a test WorkflowJobNode
func newWorkflowJobNode(name, stepType string) types.WorkflowJobNode {
	return types.WorkflowJobNode{
		WorkflowNode: types.WorkflowNode{
			Name: name,
			Type: stepType,
		},
	}
}

// newConditionNode creates a WorkflowJobNode for condition testing
func newConditionNode(name, condition string, branches map[string]string) types.WorkflowJobNode {
	return types.WorkflowJobNode{
		WorkflowNode: types.WorkflowNode{
			Name:      name,
			Type:      "condition",
			Condition: condition,
			Branches:  branches,
		},
	}
}
