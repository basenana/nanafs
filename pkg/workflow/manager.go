package workflow

import (
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
)

type Manager struct {
	workflows map[string]*Workflow
	jobs      map[string]*Job
}

func NewManager() *Manager {
	return &Manager{
		workflows: make(map[string]*Workflow),
		jobs:      make(map[string]*Job),
	}
}

func (m *Manager) CreateWorkFlow(name string, rule types.Rule, plugins []plugin.Plugin) *Workflow {
	w := NewWorkflow(name, rule, plugins)
	m.workflows[name] = w
	return w
}

func (m *Manager) UpdateWorkFlow(name string, rule types.Rule, plugins []plugin.Plugin) *Workflow {
	w, ok := m.workflows[name]
	if !ok {
		return m.CreateWorkFlow(name, rule, plugins)
	}
	w.Rule = rule
	w.Plugins = plugins
	m.workflows[name] = w
	return w
}

func (m *Manager) DeleteWorkFlow(name string) {
	delete(m.workflows, name)
}

func (m *Manager) GetWorkflows() []*Workflow {
	workflows := make([]*Workflow, len(m.workflows))
	for _, w := range m.workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

func (m *Manager) GetJobs() map[fsm.Status]*Job {
	jobs := make(map[fsm.Status]*Job)
	for _, j := range m.jobs {
		s := j.flow.status
		jobs[s] = j
	}
	return jobs
}

func (m *Manager) Trigger(o *types.Object) {
	for _, workflow := range m.workflows {
		if !workflow.Rule.Apply(o) {
			continue
		}
		job := NewJob(workflow, o)
		m.jobs[job.Id] = job
		go job.Run()
	}
}
