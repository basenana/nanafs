package workflow

import (
	"github.com/basenana/go-flow/controller"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/nanafs/config"
	ctrl "github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

type mockConfig struct{}

func (m mockConfig) GetConfig() (config.Config, error) {
	return config.Config{ApiConfig: config.Api{Enable: true}}, nil
}

var _ config.Loader = mockConfig{}

func NewControllerForTest() ctrl.Controller {
	m, _ := storage.NewMetaStorage("memory", config.Meta{})
	s, _ := storage.NewStorage("memory", config.Storage{})

	files.InitFileIoChain(config.Config{}, s, make(chan struct{}))
	return ctrl.New(mockConfig{}, m, s)
}

type fakePlugin struct {
}

func (f fakePlugin) Name() string {
	return "fake"
}

func (f fakePlugin) Run(object *types.Object) error {
	return nil
}

var _ = Describe("TestWorkflow", func() {
	var (
		ctl        ctrl.Controller
		testCtl    *controller.FlowController
		fakePlugin fakePlugin
	)
	BeforeEach(func() {
		ctl = NewControllerForTest()
		opt := controller.Option{
			Storage: FlowStorage,
		}
		flowCtl, err := controller.NewFlowController(opt)
		if err != nil {
			panic(err)
		}
		testCtl = flowCtl
		if err := testCtl.Register(&NanaJob{}); err != nil {
			panic(err)
		}
	})

	Describe("test job", func() {
		Context("job trigger", func() {
			It("should be ok", func() {
				rule := types.Rule{Logic: "", Rules: nil, Operation: nil}
				f := types.Object{}
				w := NewWorkflow("test", rule, []plugin.Plugin{fakePlugin})
				jobObj := types.Object{}
				job, _, err := NewNanaJob(ctl, w, &jobObj, &f)
				Expect(err).Should(BeNil())
				err = job.Run()
				Expect(err).Should(BeNil())
				Eventually(func() []byte {
					return status2Bytes(job.GetStatus())
				}, time.Minute*3, time.Second).Should(Equal(status2Bytes(flow.SucceedStatus)))
			})
		})
	})

})

func status2Bytes(status fsm.Status) []byte {
	return []byte(status)
}
