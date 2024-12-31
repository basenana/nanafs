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
	tasks   map[string]Task
	towards map[string]*taskToward

	// crtBatch contain current task batch need to trigger
	crtBatch []*taskToward

	hasFailed bool
	hasInited bool
	mux       sync.Mutex
}

var _ Coordinator = &DAGCoordinator{}

func NewDAGCoordinator() *DAGCoordinator {
	return &DAGCoordinator{
		tasks:   make(map[string]Task),
		towards: make(map[string]*taskToward),
	}
}

func (g *DAGCoordinator) NewTask(task Task) {
	g.tasks[task.GetName()] = task
}

func (g *DAGCoordinator) UpdateTask(task Task) {
	g.updateTaskStatus(task.GetName(), task.GetStatus())
}

func (g *DAGCoordinator) NextBatch(ctx context.Context) ([]Task, error) {
	g.mux.Lock()
	defer g.mux.Unlock()

	if !g.hasInited {
		if err := g.buildDAG(); err != nil {
			return nil, err
		}
		g.hasInited = true
	}

	next := g.nextBatchTasks()
	result := make([]Task, len(next))
	for i, n := range next {
		result[i] = n.task
	}
	return result, nil
}

func (g *DAGCoordinator) HandleFail(task Task, err error) FailOperation {
	return FailButContinue
}

func (g *DAGCoordinator) updateTaskStatus(taskName, status string) {
	g.mux.Lock()
	defer g.mux.Unlock()
	t, ok := g.towards[taskName]
	if !ok {
		return
	}
	t.status = status
	g.towards[taskName] = t
	if status == FailedStatus || status == ErrorStatus {
		g.hasFailed = true
	}
	return
}

func (g *DAGCoordinator) nextBatchTasks() []*taskToward {
	for _, t := range g.crtBatch {
		task := g.towards[t.taskName]
		if !IsFinishedStatus(task.status) {
			return g.crtBatch
		}
	}

	nextBatch := make([]*taskToward, 0)
	for _, t := range g.crtBatch {
		if t.status == SucceedStatus && t.onSucceed != "" {
			nextBatch = append(nextBatch, g.towards[t.onSucceed])
		}
		if (t.status == FailedStatus || t.status == ErrorStatus) && t.onFailed != "" {
			nextBatch = append(nextBatch, g.towards[t.onFailed])
		}
	}

	if len(nextBatch) != 0 {
		g.crtBatch = nextBatch
		return g.crtBatch
	}

	return nil
}

func (g *DAGCoordinator) buildDAG() error {
	for tname, t := range g.tasks {
		if _, exist := g.towards[tname]; exist {
			return fmt.Errorf("duplicate task %s definition", t.GetName())
		}

		director, ok := t.(directorWrapper)
		if ok {
			next := director.NextTask
			g.towards[tname] = &taskToward{
				taskName:  tname,
				status:    t.GetStatus(),
				onSucceed: next.OnSucceed,
				onFailed:  next.OnFailed,
				task:      director.Task,
			}
			continue
		}

		g.towards[tname] = &taskToward{
			taskName: tname,
			status:   t.GetStatus(),
			task:     g.tasks[tname],
		}
	}

	for _, t := range g.towards {
		if t.onSucceed != "" {
			if _, exist := g.towards[t.onSucceed]; !exist {
				return fmt.Errorf("next task after %s succeed(%s) is does not define", t.taskName, t.onSucceed)
			}
		}
		if t.onFailed != "" {
			if _, exist := g.towards[t.onFailed]; !exist {
				return fmt.Errorf("next task after %s failed(%s) is does not define", t.taskName, t.onFailed)
			}
		}
	}

	firstTaskName, err := newDagChecker(g.towards).firstBatch()
	if err != nil {
		return err
	}

	firstTaskNameSet := NewStringSet(firstTaskName...)
	for i, t := range g.towards {
		if firstTaskNameSet.Has(t.taskName) {
			g.crtBatch = append(g.crtBatch, g.towards[i])
		}
	}

	return nil
}

type NextTask struct {
	OnSucceed string
	OnFailed  string
}

type directorWrapper struct {
	Task
	NextTask
}

func WithDirector(task Task, nextTask NextTask) Task {
	return directorWrapper{
		Task:     task,
		NextTask: nextTask,
	}
}

type taskToward struct {
	taskName  string
	status    string
	onSucceed string
	onFailed  string
	task      Task
}

type taskDep struct {
	taskSet   StringSet
	taskEdges map[string][]string
	preCount  map[string]int
}

func (t *taskDep) firstBatch() ([]string, error) {
	batches := make([][]string, 0)
	for {
		if t.taskSet.Len() == 0 {
			break
		}

		batch := make([]string, 0)
		for _, task := range t.taskSet.List() {
			if t.preCount[task] == 0 {
				batch = append(batch, task)
			}

			t.taskSet.Del(task)
		}

		if len(batch) == 0 {
			return nil, fmt.Errorf("there is a loop in the diagram")
		}
		batches = append(batches, batch)

		for _, task := range batch {
			nextTasks := t.taskEdges[task]
			for _, nt := range nextTasks {
				t.preCount[nt] -= 1
			}
		}
	}
	if len(batches) == 0 {
		return nil, fmt.Errorf("there are no executable nodes in the diagram")
	}
	return batches[0], nil
}

func (t *taskDep) order(firstTask string, nextTasks []string) {
	t.taskSet.Insert(firstTask)
	for _, task := range nextTasks {
		t.taskSet.Insert(task)

		edges, ok := t.taskEdges[firstTask]
		if !ok {
			edges = make([]string, 0)
		}

		exist := false
		for _, d := range edges {
			if d == task {
				exist = true
				break
			}
		}
		if exist {
			continue
		}
		edges = append(edges, task)
		t.taskEdges[firstTask] = edges
		t.preCount[task] += 1
	}
}

func newDagChecker(tasks map[string]*taskToward) *taskDep {
	c := &taskDep{
		taskSet:   NewStringSet(),
		taskEdges: map[string][]string{},
		preCount:  map[string]int{},
	}

	for _, t := range tasks {
		nextTask := make([]string, 0, 2)
		if t.onSucceed != "" {
			nextTask = append(nextTask, t.onSucceed)
		}
		if t.onFailed != "" {
			nextTask = append(nextTask, t.onFailed)
		}
		c.order(t.taskName, nextTask)
	}

	return c
}
