package workflow

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hyponet/eventbus/bus"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	sync.RWMutex
	logger *zap.SugaredLogger
	ctrl   controller.Controller
	meta   storage.MetaStore

	wfParent  *types.Object
	jobParent *types.Object
	workflows map[string]*Workflow
}

func NewWorkflowManager(ctrl controller.Controller, meta storage.MetaStore) (*Manager, error) {
	root, err := ctrl.LoadRootObject(context.TODO())
	if err != nil {
		return nil, err
	}
	wfParent, err := ctrl.FindObject(context.TODO(), root, ".workflow")
	if err != nil {
		return nil, err
	}
	jobParent, err := ctrl.FindObject(context.TODO(), root, ".job")
	if err != nil {
		return nil, err
	}
	wfs, err := ctrl.ListObjectChildren(context.TODO(), wfParent)
	if err != nil {
		return nil, err
	}

	wfMaps := make(map[string]*Workflow)
	for _, o := range wfs {
		w := &types.Workflow{}
		err = ctrl.LoadStructureObject(context.TODO(), o, w)
		if err != nil {
			return nil, err
		}
		plugins := make([]plugin.Plugin, 0)
		for _, a := range w.Actions {
			if p, ok := plugin.Plugins[a]; ok {
				plugins = append(plugins, p)
			}
		}
		wfMaps[o.ID] = &Workflow{
			obj:     *o,
			Name:    o.Name,
			Rule:    w.Rule.ToRule(),
			Plugins: plugins,
		}
	}
	return &Manager{
		ctrl:      ctrl,
		meta:      meta,
		logger:    logger.NewLogger("WorkflowManager"),
		wfParent:  wfParent,
		jobParent: jobParent,
		workflows: wfMaps,
	}, nil
}

func (m *Manager) Run(ctx context.Context) error {
	if _, err := bus.Subscribe("object.workflow.*.close", m.WorkFlowHandler); err != nil {
		return err
	}
	if _, err := bus.Subscribe("object.file.*.close", m.FileSaveHandler); err != nil {
		return err
	}
	if _, err := bus.Subscribe("object.entry.*.destroy", m.FileDeleteHandler); err != nil {
		return err
	}
	if _, err := bus.Subscribe("object.workflow.*.destroy", m.WorkFlowDestroyHandler); err != nil {
		return err
	}

	go func() {
		d := time.Duration(time.Second * 10)
		ticker := time.NewTicker(d)

		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				then := time.Now().Add(-10 * time.Minute)
				filter := types.WorkflowFilter{
					Synced:    false,
					UpdatedAt: &then,
				}
				objWF, err := m.meta.ListObjectWorkflows(ctx, filter)
				if err != nil {
					return
				}
				for _, o := range objWF {
					obj, err := m.meta.GetObject(ctx, o.Id)
					if err != nil {
						continue
					}
					m.handle(obj)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
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

	objWF := types.ObjectWorkflow{
		Id:     obj.ID,
		Synced: false,
	}
	err := m.meta.SaveObjectWorkflow(context.TODO(), &objWF)
	if err != nil {
		m.logger.Errorf("save object workflow error: %v", err)
	}
}

func (m *Manager) FileDeleteHandler(obj *types.Object) {
	if m.skipFiles(obj) {
		return
	}

	err := m.meta.DeleteObjectWorkflow(context.TODO(), obj.ID)
	if err != nil {
		m.logger.Errorf("delete object workflow error: %v", err)
	}
}

func (m *Manager) handle(obj *types.Object) {
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
			Name:   fmt.Sprintf("%s-%s-job-%s", obj.Name, w.Name, utils.RandStringRunes(6)),
			Kind:   types.JobKind,
			Access: m.jobParent.Access,
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

	objWF := types.ObjectWorkflow{
		Id:     obj.ID,
		Synced: true,
	}
	err := m.meta.SaveObjectWorkflow(context.TODO(), &objWF)
	if err != nil {
		m.logger.Errorf("save object workflow error: %v", err)
		return
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
