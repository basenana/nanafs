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

package jobrun

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"sync"
)

type stepToward struct {
	stepName  string
	status    string
	onSucceed string
	onFailed  string
}

type DAG struct {
	// steps contain all task
	steps map[string]*stepToward

	// crtBatch contain current task batch need to trigger
	crtBatch []*stepToward

	hasFailed bool
	mux       sync.Mutex
}

func (g *DAG) updateTaskStatus(taskName, status string) {
	g.mux.Lock()
	defer g.mux.Unlock()
	t, ok := g.steps[taskName]
	if !ok {
		return
	}
	t.status = status
	g.steps[taskName] = t
	if status == FailedStatus || status == ErrorStatus {
		g.hasFailed = true
	}
	return
}

func (g *DAG) nextBatchTasks() []*stepToward {
	g.mux.Lock()
	defer g.mux.Unlock()
	for _, t := range g.crtBatch {
		task := g.steps[t.stepName]
		if !IsFinishedStatus(task.status) {
			return g.crtBatch
		}
	}

	nextBatch := make([]*stepToward, 0)
	for _, t := range g.crtBatch {
		if t.status == SucceedStatus && t.onSucceed != "" {
			nextBatch = append(nextBatch, g.steps[t.onSucceed])
		}
		if (t.status == FailedStatus || t.status == ErrorStatus) && t.onFailed != "" {
			nextBatch = append(nextBatch, g.steps[t.onFailed])
		}
	}

	if len(nextBatch) != 0 {
		g.crtBatch = nextBatch
		return g.crtBatch
	}

	return nil
}

func buildDAG(steps []types.WorkflowJobStep) (*DAG, error) {
	dag := &DAG{steps: map[string]*stepToward{}}

	for i, t := range steps {
		if _, exist := dag.steps[t.StepName]; exist {
			return nil, fmt.Errorf("duplicate task %s definition", t.StepName)
		}
		dag.steps[t.StepName] = &stepToward{
			stepName: t.StepName,
			status:   t.Status,
		}

		// TODO: support DAG task define
		if i+1 < len(steps) {
			dag.steps[t.StepName].onSucceed = steps[i+1].StepName
		}
	}

	for _, t := range dag.steps {
		if t.onSucceed != "" {
			if _, exist := dag.steps[t.onSucceed]; !exist {
				return nil, fmt.Errorf("next task after %s succeed(%s) is does not define", t.stepName, t.onSucceed)
			}
		}
		if t.onFailed != "" {
			if _, exist := dag.steps[t.onFailed]; !exist {
				return nil, fmt.Errorf("next task after %s failed(%s) is does not define", t.stepName, t.onFailed)
			}
		}
	}

	firstTaskName, err := newDagChecker(dag.steps).firstBatch()
	if err != nil {
		return nil, err
	}

	firstTaskNameSet := utils.NewStringSet(firstTaskName...)
	for i, t := range dag.steps {
		if firstTaskNameSet.Has(t.stepName) {
			dag.crtBatch = append(dag.crtBatch, dag.steps[i])
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

func newDagChecker(tasks map[string]*stepToward) *taskDep {
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
		c.order(t.stepName, nextTask)
	}

	return c
}
