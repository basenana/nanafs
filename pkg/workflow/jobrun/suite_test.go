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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	testcfg "github.com/onsi/ginkgo/config"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	tempDir    string
	recorder   metastore.ScheduledTaskRecorder
	notifyImpl *notify.Notify
	jobCtrl    *Controller

	closeCtrlFn context.CancelFunc
)

func TestJobRun(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	testcfg.DefaultReporterConfig.SlowSpecThreshold = time.Minute.Seconds()

	RunSpecs(t, "Workflow JobRun Suite")
}

var _ = BeforeSuite(func() {
	var err error
	tempDir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-jobrun-")
	Expect(err).Should(BeNil())

	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())
	recorder = memMeta
	storage.InitLocalCache(config.Config{CacheDir: tempDir, CacheSize: 1})

	notifyImpl = notify.NewNotify(memMeta)

	// init plugin
	Expect(plugin.Init(&config.Plugin{})).Should(BeNil())

	// init fake workflow to test wf job
	fakeWf := &types.WorkflowSpec{
		Id:   "fake-workflow-1",
		Name: "fake-workflow-1",
	}
	err = recorder.SaveWorkflow(context.TODO(), fakeWf)
	Expect(err).Should(BeNil())

	jobCtrl = NewJobController(recorder, notifyImpl)
	var ctx context.Context
	ctx, closeCtrlFn = context.WithCancel(context.Background())
	jobCtrl.Start(ctx)
})

var _ = AfterSuite(func() {
	closeCtrlFn()
})

func init() {
	// fake executor
	defaultExecName = "fake"
	RegisterExecutorBuilder(defaultExecName, func(job *types.WorkflowJob) Executor { return &fakeExecutor{} })
}

type fakeExecutor struct{}

var _ Executor = &fakeExecutor{}

func (f *fakeExecutor) Setup(ctx context.Context) error { return nil }

func (f *fakeExecutor) DoOperation(ctx context.Context, step types.WorkflowJobStep) error {
	time.Sleep(time.Second * 5)
	return nil
}

func (f *fakeExecutor) Collect(ctx context.Context) error { return nil }

func (f *fakeExecutor) Teardown(ctx context.Context) { return }
