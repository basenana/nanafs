/*
   Copyright 2023 Go-Flow Authors

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
	"fmt"
	"github.com/basenana/go-flow/utils"
	"sync"
)

type taskToward struct {
	taskName        string
	status          string
	onSucceed       string
	onFailed        string
	activeOnFailure bool
}

type DAG struct {
	// tasks contain all task
	tasks map[string]taskToward

	// crtBatch contain current task batch need to trigger
	crtBatch []taskToward

	// onFailure contain all the tasks that need to be executed after a task failure
	onFailure []taskToward

	hasFailed bool
	mux       sync.Mutex
}

func (g *DAG) updateTaskStatus(taskName, status string) {
	g.mux.Lock()
	defer g.mux.Unlock()
	t, ok := g.tasks[taskName]
	if !ok {
		return
	}
	t.status = status
	g.tasks[taskName] = t
	if status == FailedStatus || status == ErrorStatus {
		g.hasFailed = true
	}
	return
}

func (g *DAG) nextBatchTasks() []taskToward {
	g.mux.Lock()
	defer g.mux.Unlock()
	for _, t := range g.crtBatch {
		task := g.tasks[t.taskName]
		if !IsFinishedStatus(task.status) {
			return g.crtBatch
		}
	}

	nextBatch := make([]taskToward, 0)
	for _, t := range g.crtBatch {
		if t.status == SucceedStatus && t.onSucceed != "" {
			nextBatch = append(nextBatch, g.tasks[t.onSucceed])
		}
		if (t.status == FailedStatus || t.status == ErrorStatus) && t.onFailed != "" {
			nextBatch = append(nextBatch, g.tasks[t.onFailed])
		}
	}

	if len(nextBatch) != 0 {
		g.crtBatch = nextBatch
		return g.crtBatch
	}

	if g.hasFailed && len(g.onFailure) > 0 {
		g.crtBatch = g.onFailure
		g.onFailure = nil
		return g.crtBatch
	}

	return nil
}

func buildDAG(tasks []Task) (*DAG, error) {
	dag := &DAG{tasks: map[string]taskToward{}}

	for _, t := range tasks {
		if _, exist := dag.tasks[t.Name]; exist {
			return nil, fmt.Errorf("duplicate task %s definition", t.Name)
		}
		dag.tasks[t.Name] = taskToward{
			taskName:        t.Name,
			status:          t.Status,
			onSucceed:       t.Next.OnSucceed,
			onFailed:        t.Next.OnFailed,
			activeOnFailure: t.ActiveOnFailure,
		}
	}

	for _, t := range dag.tasks {
		if t.onSucceed != "" {
			if _, exist := dag.tasks[t.onSucceed]; !exist {
				return nil, fmt.Errorf("next task after %s succeed(%s) is does not define", t.taskName, t.onSucceed)
			}
		}
		if t.onFailed != "" {
			if _, exist := dag.tasks[t.onFailed]; !exist {
				return nil, fmt.Errorf("next task after %s failed(%s) is does not define", t.taskName, t.onFailed)
			}
		}
	}

	firstTaskName, err := newDagChecker(dag.tasks).firstBatch()
	if err != nil {
		return nil, err
	}

	firstTaskNameSet := utils.NewStringSet(firstTaskName...)
	for i, t := range dag.tasks {
		if firstTaskNameSet.Has(t.taskName) {
			if t.activeOnFailure {
				dag.onFailure = append(dag.onFailure, dag.tasks[i])
				continue
			}
			dag.crtBatch = append(dag.crtBatch, dag.tasks[i])
		}
	}

	return dag, nil
}

type taskDep struct {
	taskSet   utils.StringSet
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

func newDagChecker(tasks map[string]taskToward) *taskDep {
	c := &taskDep{
		taskSet:   utils.NewStringSet(),
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
