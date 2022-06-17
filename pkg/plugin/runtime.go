package plugin

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"sync"
)

type run struct {
	id     string
	p      types.Plugin
	obj    *types.Object
	config map[string]string
	stopCh chan struct{}
}

type runtime struct {
	pluginRuns   map[string]*run
	globalStopCh chan struct{}
	mux          sync.Mutex
	logger       *zap.SugaredLogger
}

func (r *runtime) trigger(obj *types.Object, p types.Plugin, config map[string]string) {
	pRun := run{
		id:     fmt.Sprintf("%s-%s", p.Name(), uuid.New().String()),
		obj:    obj,
		p:      p,
		config: config,
		stopCh: make(chan struct{}),
	}

	r.mux.Lock()
	r.pluginRuns[pRun.id] = &pRun
	r.mux.Unlock()
	go r.runOnePlugin(&pRun)
}

func (r *runtime) runOnePlugin(pRun *run) {

}
