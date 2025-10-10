/*
   Copyright 2024 Go-Flow Authors

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

package flow

import (
	"context"
	"fmt"
	"sync"
)

type DAGCoordinator struct {
	// tasks contain all task
	tasks map[string]*BasicTask

	// graph
	pendingTasks StringSet
	taskEdges    map[string][]string
	preCount     map[string]int

	failOP FailOperation
	mux    sync.Mutex
}

var _ Coordinator = &DAGCoordinator{}

func NewDAGCoordinator(options ...CoordinatorOption) *DAGCoordinator {
	opt := &CoordinatorOptions{}
	for _, optFn := range options {
		optFn(opt)
	}

	if opt.failOP == "" {
		opt.failOP = FailAndInterrupt
	}

	return &DAGCoordinator{
		tasks:        make(map[string]*BasicTask),
		pendingTasks: NewStringSet(),
		taskEdges:    map[string][]string{},
		preCount:     map[string]int{},
		failOP:       opt.failOP,
	}
}

func (g *DAGCoordinator) NewTask(task Task) {
	var (
		tt = &BasicTask{
			Name:   task.GetName(),
			Status: task.GetStatus(),
		}
		depends []string
	)

	wrapper, ok := task.(*dependentWrapper)
	if ok {
		depends = wrapper.depends
	}

	g.mux.Lock()
	g.tasks[tt.Name] = tt
	g.dependTasks(tt.Name, depends)
	g.mux.Unlock()
}

func (g *DAGCoordinator) UpdateTask(task Task) {
	g.updateTaskStatus(task.GetName(), task.GetStatus())
}

func (g *DAGCoordinator) Finished() bool {
	return g.pendingTasks.Len() == 0
}

func (g *DAGCoordinator) HandleFail(task Task, err error) FailOperation {
	return g.failOP
}

func (g *DAGCoordinator) updateTaskStatus(taskName, status string) {
	g.mux.Lock()
	t, ok := g.tasks[taskName]
	g.mux.Unlock()
	if !ok {
		return
	}
	t.Status = status
	return
}

func (g *DAGCoordinator) NextBatch(ctx context.Context) ([]string, error) {
	if g.pendingTasks.Len() == 0 {
		return nil, nil
	}

	g.mux.Lock()
	defer g.mux.Unlock()

	var (
		batch   = make([]string, 0)
		skipped = false
	)
	for _, task := range g.pendingTasks.List() {
		if g.preCount[task] != 0 {
			continue
		}

		t, ok := g.tasks[task]
		if !ok {
			return nil, fmt.Errorf("task %s not found", task)
		}
		if t.Status == PausedStatus {
			skipped = true
			continue
		}

		batch = append(batch, task)
		g.pendingTasks.Del(task)
	}

	if len(batch) == 0 {
		if skipped {
			return nil, nil // some task cannot run right now
		}
		return nil, fmt.Errorf("there is a loop in the diagram")
	}

	for _, task := range batch {
		nextTasks := g.taskEdges[task]
		for _, nt := range nextTasks {
			g.preCount[nt] -= 1
		}
	}

	return batch, nil
}

func (g *DAGCoordinator) dependTasks(nextTask string, dependTasks []string) {
	g.pendingTasks.Insert(nextTask)
	for _, task := range dependTasks {
		g.pendingTasks.Insert(task)

		edges, ok := g.taskEdges[task]
		if !ok {
			edges = make([]string, 0)
		}

		exist := false
		for _, d := range edges {
			if d == nextTask {
				exist = true
				break
			}
		}
		if exist {
			continue
		}
		edges = append(edges, nextTask)
		g.taskEdges[task] = edges
		g.preCount[nextTask] += 1
	}
}

type dependentWrapper struct {
	Task
	depends []string
}

func (w *dependentWrapper) unwrapped() Task {
	return w.Task
}

func WithDependent(task Task, depends []string) Task {
	return &dependentWrapper{
		Task:    task,
		depends: depends,
	}
}
