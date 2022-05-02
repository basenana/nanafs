package workflow

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hyponet/eventbus/bus"
	"go.uber.org/zap"
	"strings"
	"sync"
)

type Manager struct {
	sync.RWMutex
	logger *zap.SugaredLogger
	ctrl   controller.Controller

	wfParent  *types.Object
	jobParent *types.Object
	workflows map[string]*Workflow
}

func NewWorkflowManager(ctrl controller.Controller) (*Manager, error) {
	root, err := ctrl.LoadRootObject(context.TODO())
	if err != nil {
		return nil, err
	}
	wf, err := ctrl.FindObject(context.TODO(), root, ".workflow")
	if err != nil {
		return nil, err
	}
	job, err := ctrl.FindObject(context.TODO(), root, ".job")
	if err != nil {
		return nil, err
	}
	return &Manager{
		ctrl:      ctrl,
		logger:    logger.NewLogger("WorkflowManager"),
		wfParent:  wf,
		jobParent: job,
		workflows: map[string]*Workflow{},
	}, nil
}

func (m *Manager) Run() error {
	_, err := bus.Subscribe("object.workflow.*.close", m.WorkFlowHandler)
	if err != nil {
		return err
	}
	_, err = bus.Subscribe("object.file.*.close", m.FileSaveHandler)
	if err != nil {
		return err
	}

	_, err = bus.Subscribe("object.workflow.*.destroy", m.WorkFlowDestroyHandler)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) WorkFlowHandler(obj *types.Object) {
	wf := &types.Workflow{}
	err := m.ctrl.LoadStructureObject(context.TODO(), obj, wf)
	if err != nil {
		m.logger.Errorw("load workflow content error: %v", err)
		return
	}
	if wf == nil {
		return
	}
	plugins := make([]plugin.Plugin, 0)
	for _, a := range wf.Actions {
		if p, ok := plugin.Plugins[a]; ok {
			plugins = append(plugins, p)
		}
	}
	m.Lock()
	defer m.Unlock()
	if _, ok := m.workflows[obj.ID]; !ok {
		m.workflows[obj.ID] = &Workflow{
			obj:     *obj,
			Name:    wf.Name,
			Rule:    wf.Rule.ToRule(),
			Plugins: plugins,
		}
	}
}

func (m *Manager) WorkFlowDestroyHandler(obj *types.Object) {
	if _, ok := m.workflows[obj.ID]; !ok {
		return
	}
	m.Lock()
	defer m.Unlock()
	delete(m.workflows, obj.ID)
}

func (m *Manager) FileSaveHandler(obj *types.Object) {
	if m.skipFiles(obj) {
		return
	}
	m.RLock()
	workflows := m.workflows
	m.RUnlock()

	for _, w := range workflows {
		if !w.Rule.Apply(obj) {
			continue
		}
		attr := types.ObjectAttr{
			Name: fmt.Sprintf("%s-%s-job-%s", obj.Name, w.Name, utils.RandStringRunes(6)),
			Kind: types.JobKind,
		}
		jobObj, err := m.ctrl.CreateObject(context.TODO(), m.jobParent, attr)
		if err != nil {
			m.logger.Errorw("create job obj error: %v", err)
			return
		}
		job, content, err := NewNanaJob(m.ctrl, w, jobObj, obj)
		if err != nil {
			m.logger.Errorw("new nanajob error: %v", err)
			return
		}
		if err := m.ctrl.SaveStructureObject(context.TODO(), jobObj, content); err != nil {
			m.logger.Errorw("save job content error: %v", err)
			return
		}
		go func() {
			err := job.Run()
			if err != nil {
				m.logger.Errorw("job %s runs error: %v", job.Id, err)
			}
		}()
	}
}

func (m *Manager) skipFiles(obj *types.Object) bool {
	if strings.HasPrefix(obj.Name, ".") {
		return true
	}
	if m.ctrl.IsStructured(obj) {
		return true
	}
	return false
}
