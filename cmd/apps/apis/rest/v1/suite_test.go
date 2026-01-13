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

package v1

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/cmd/apps/apis/rest/common"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow"
)

var (
	testMeta   metastore.Meta
	testCfg    *config.Bootstrap
	testRouter *TestRouter
)

// TestRouter provides a pre-configured gin router for testing
type TestRouter struct {
	Router       *gin.Engine
	Services     *ServicesV1
	MockWorkflow *MockWorkflow
}

func NewTestRouter() *TestRouter {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	cfg := config.NewMockConfig(*testCfg)
	dep, err := common.InitDepends(cfg, testMeta)
	if err != nil {
		panic(err)
	}

	mockWorkflow := NewMockWorkflow()

	services := &ServicesV1{
		meta:     dep.Meta,
		core:     dep.Core,
		workflow: mockWorkflow,
		notify:   dep.Notify,
		cfg:      dep.Config,
		logger:   logger.NewLogger("rest-test"),
	}

	router.Use(common.AuthMiddleware(testCfg.API.JWT))
	RegisterRoutes(router, services)

	return &TestRouter{
		Router:       router,
		Services:     services,
		MockWorkflow: mockWorkflow,
	}
}

// MockWorkflow implements workflow.Workflow interface for testing
type MockWorkflow struct {
	workflows map[string]*types.Workflow
	jobs      map[string]*types.WorkflowJob
}

func NewMockWorkflow() *MockWorkflow {
	return &MockWorkflow{
		workflows: make(map[string]*types.Workflow),
		jobs:      make(map[string]*types.WorkflowJob),
	}
}

func (m *MockWorkflow) Start(ctx context.Context) {}

func (m *MockWorkflow) ListWorkflows(ctx context.Context, namespace string) ([]*types.Workflow, error) {
	result := make([]*types.Workflow, 0, len(m.workflows))
	for _, w := range m.workflows {
		if w.Namespace == namespace {
			result = append(result, w)
		}
	}
	return result, nil
}

func (m *MockWorkflow) GetWorkflow(ctx context.Context, namespace string, wfId string) (*types.Workflow, error) {
	w, ok := m.workflows[wfId]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", wfId)
	}
	if w.Namespace != namespace {
		return nil, fmt.Errorf("workflow not found: %s", wfId)
	}
	return w, nil
}

func (m *MockWorkflow) CreateWorkflow(ctx context.Context, namespace string, spec *types.Workflow) (*types.Workflow, error) {
	spec.Namespace = namespace
	spec.Id = fmt.Sprintf("wf-%d", len(m.workflows)+1)
	spec.CreatedAt = time.Now()
	spec.UpdatedAt = time.Now()
	m.workflows[spec.Id] = spec
	return spec, nil
}

func (m *MockWorkflow) UpdateWorkflow(ctx context.Context, namespace string, spec *types.Workflow) (*types.Workflow, error) {
	if _, ok := m.workflows[spec.Id]; !ok {
		return nil, fmt.Errorf("workflow not found: %s", spec.Id)
	}
	spec.Namespace = namespace
	spec.UpdatedAt = time.Now()
	m.workflows[spec.Id] = spec
	return spec, nil
}

func (m *MockWorkflow) DeleteWorkflow(ctx context.Context, namespace string, wfId string) error {
	if _, ok := m.workflows[wfId]; !ok {
		return fmt.Errorf("workflow not found: %s", wfId)
	}
	delete(m.workflows, wfId)
	return nil
}

func (m *MockWorkflow) ListJobs(ctx context.Context, namespace string, wfId string) ([]*types.WorkflowJob, error) {
	result := make([]*types.WorkflowJob, 0)
	for _, j := range m.jobs {
		if j.Namespace == namespace && j.Workflow == wfId {
			result = append(result, j)
		}
	}
	return result, nil
}

func (m *MockWorkflow) GetJob(ctx context.Context, namespace string, wfId string, jobID string) (*types.WorkflowJob, error) {
	j, ok := m.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	if j.Namespace != namespace || j.Workflow != wfId {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	return j, nil
}

func (m *MockWorkflow) TriggerWorkflow(ctx context.Context, namespace string, wfId string, tgt types.WorkflowTarget, attr workflow.JobAttr) (*types.WorkflowJob, error) {
	w, ok := m.workflows[wfId]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", wfId)
	}
	if w.Namespace != namespace {
		return nil, fmt.Errorf("workflow not found: %s", wfId)
	}

	job := &types.WorkflowJob{
		Id:            fmt.Sprintf("job-%d", len(m.jobs)+1),
		Namespace:     namespace,
		Workflow:      wfId,
		TriggerReason: attr.Reason,
		Targets:       tgt,
		Parameters:    attr.Parameters,
		Status:        "running",
		QueueName:     w.QueueName,
		StartAt:       time.Now(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	m.jobs[job.Id] = job
	return job, nil
}

func (m *MockWorkflow) PauseWorkflowJob(ctx context.Context, namespace string, jobId string) error {
	j, ok := m.jobs[jobId]
	if !ok {
		return fmt.Errorf("job not found: %s", jobId)
	}
	if j.Status != "running" {
		return fmt.Errorf("pausing is not supported in non-running state")
	}
	j.Status = "paused"
	j.UpdatedAt = time.Now()
	return nil
}

func (m *MockWorkflow) ResumeWorkflowJob(ctx context.Context, namespace string, jobId string) error {
	j, ok := m.jobs[jobId]
	if !ok {
		return fmt.Errorf("job not found: %s", jobId)
	}
	if j.Status != "paused" {
		return fmt.Errorf("resuming is not supported in non-paused state")
	}
	j.Status = "running"
	j.UpdatedAt = time.Now()
	return nil
}

func (m *MockWorkflow) CancelWorkflowJob(ctx context.Context, namespace string, jobId string) error {
	j, ok := m.jobs[jobId]
	if !ok {
		return fmt.Errorf("job not found: %s", jobId)
	}
	if !j.FinishAt.IsZero() {
		return fmt.Errorf("canceling is not supported in finished state")
	}
	j.Status = "canceled"
	j.FinishAt = time.Now()
	j.UpdatedAt = time.Now()
	return nil
}

func TestRestV1API(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "REST V1 Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())
	testMeta = memMeta

	workdir, err := os.MkdirTemp(os.TempDir(), "ut-nanafs-rest-")
	Expect(err).Should(BeNil())

	testCfg = &config.Bootstrap{
		API: config.FsApi{
			JWT: nil, // No JWT configured, test header-based auth (backward compatibility)
		},
		FS:        &config.FS{Owner: config.FSOwner{Uid: 0, Gid: 0}, Writeback: false},
		Meta:      config.Meta{Type: metastore.MemoryMeta},
		Storages:  []config.Storage{{ID: "test-memory-0", Type: storage.MemoryStorage}},
		Workflow:  config.Workflow{JobWorkdir: path.Join(workdir, "job-workdir")},
		CacheDir:  workdir,
		CacheSize: 0,
	}

	cfg := config.NewMockConfig(*testCfg)
	dep, err := common.InitDepends(cfg, memMeta)
	Expect(err).Should(BeNil())

	// init root
	_, err = dep.Core.FSRoot(context.TODO())
	Expect(err).Should(BeNil())

	// init default namespace
	err = dep.Core.CreateNamespace(context.TODO(), types.DefaultNamespace)
	Expect(err).Should(BeNil())
})

var _ = BeforeEach(func() {
	// Create a fresh test router for each test to ensure isolation
	testRouter = NewTestRouter()
	// Ensure namespace exists for this Core instance
	err := testRouter.Services.core.CreateNamespace(context.TODO(), types.DefaultNamespace)
	if err != nil {
		println("BeforeEach: CreateNamespace ERROR:", err.Error())
	}
})

func BoolPtr(b bool) *bool {
	return &b
}
