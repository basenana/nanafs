package workflow

import (
	"github.com/basenana/go-flow/controller"
	"github.com/basenana/go-flow/storage"
)

var (
	FlowCtl     *controller.FlowController
	FlowStorage storage.Interface
	FlowManager *Manager
)

func init() {
	FlowStorage = storage.NewInMemoryStorage()
	opt := controller.Option{
		Storage: FlowStorage,
	}
	ctl, err := controller.NewFlowController(opt)
	if err != nil {
		panic(err)
	}
	FlowCtl = ctl
	if err := FlowCtl.Register(&NanaFlow{}); err != nil {
		panic(err)
	}
	FlowManager = NewManager()
}
