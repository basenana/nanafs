package workflow

import (
	"github.com/basenana/go-flow/controller"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/nanafs/pkg/object"
	"github.com/basenana/nanafs/pkg/plugin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

type fakePlugin struct {
}

func (f fakePlugin) Name() string {
	return "fake"
}

func (f fakePlugin) Run(object object.Object) error {
	return nil
}

var _ = Describe("TestWorkflow", func() {
	var (
		testCtl    *controller.FlowController
		fakePlugin fakePlugin
	)
	BeforeEach(func() {
		opt := controller.Option{
			Storage: FlowStorage,
		}
		ctl, err := controller.NewFlowController(opt)
		if err != nil {
			panic(err)
		}
		testCtl = ctl
		if err := testCtl.Register(&NanaFlow{}); err != nil {
			panic(err)
		}
	})

	Describe("test job", func() {
		Context("job trigger", func() {
			It("should be ok", func() {
				rule := object.Rule{Logic: "", Rules: nil, Operation: nil}
				f := fileObject{}
				w := NewWorkflow("test", rule, []plugin.Plugin{fakePlugin})
				j := NewJob(w, f)
				err := j.Run()
				Expect(err).Should(BeNil())
				Eventually(func() []byte {
					return status2Bytes(j.flow.GetStatus())
				}, time.Minute*3, time.Second).Should(Equal(status2Bytes(flow.SucceedStatus)))
			})
		})
	})

})

func status2Bytes(status fsm.Status) []byte {
	return []byte(status)
}

type fileObject struct {
	ID string
}

func (f fileObject) GetObjectMeta() object.Metadata {
	return object.Metadata{
		ID: f.ID,
	}
}

func (f fileObject) GetExtendData() object.ExtendData {
	//TODO implement me
	panic("implement me")
}

func (f fileObject) GetCustomColumn() object.CustomColumn {
	//TODO implement me
	panic("implement me")
}
