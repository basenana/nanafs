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
	"reflect"
	"sort"
	"testing"
)

func TestDAG_nextBatchTasks(t *testing.T) {
	tests := []struct {
		name        string
		dag         *DAG
		taskBatches [][]string
	}{
		{
			name: "default-1",
			dag: mustBuildDAG([]Task{
				{Name: "task-1", Next: NextTask{}},
				{Name: "task-2", Next: NextTask{}},
				{Name: "task-3", Next: NextTask{}},
			}),
			taskBatches: [][]string{{"task-1", "task-2", "task-3"}},
		},
		{
			name: "default-2",
			dag: mustBuildDAG([]Task{
				{Name: "task-1", Next: NextTask{}},
				{Name: "task-2", Next: NextTask{}},
			}),
			taskBatches: [][]string{{"task-1", "task-2"}},
		},
		{
			name: "sequential-1",
			dag: mustBuildDAG([]Task{
				{Name: "task-1", Next: NextTask{OnSucceed: "task-2"}},
				{Name: "task-2", Next: NextTask{OnSucceed: "task-3"}},
				{Name: "task-3", Next: NextTask{}},
			}),
			taskBatches: [][]string{{"task-1"}, {"task-2"}, {"task-3"}},
		},
		{
			name: "sequential-2",
			dag: mustBuildDAG([]Task{
				{Name: "task-1", Next: NextTask{OnSucceed: "task-3", OnFailed: "task-2"}},
				{Name: "task-2", Next: NextTask{}},
				{Name: "task-3", Next: NextTask{}},
			}),
			taskBatches: [][]string{{"task-1"}, {"task-3"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchNum := 0
			for {
				taskBatch := tt.dag.nextBatchTasks()
				if taskBatch == nil {
					break
				}
				if !isMarchBatchTasks(taskBatch, tt.taskBatches[batchNum]) {
					t.Errorf("nextBatchTasks() = %v, want %v", taskBatch, tt.taskBatches[batchNum])
				}
				for _, t := range taskBatch {
					tt.dag.updateTaskStatus(t.taskName, SucceedStatus)
				}
				batchNum += 1
			}
		})
	}
}

func isMarchBatchTasks(crt []*taskToward, expect []string) bool {
	var current []string
	for _, t := range crt {
		current = append(current, t.taskName)
	}

	sort.Strings(current)
	sort.Strings(expect)
	return reflect.DeepEqual(current, expect)
}

func mustBuildDAG(tasks []Task) *DAG {
	dag, err := buildDAG(tasks)
	if err != nil {
		panic(err)
	}
	return dag
}
