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

import "context"

type Coordinator interface {
	NewTask(task Task)
	UpdateTask(task Task)
	NextBatch(ctx context.Context) ([]Task, error)
	HandleFail(task Task, err error) FailOperation
}

type pipelineCoordinator struct {
	idx   int
	tasks []Task
	op    FailOperation
}

func (p *pipelineCoordinator) NewTask(task Task) {
	p.tasks = append(p.tasks, task)
}

func (p *pipelineCoordinator) UpdateTask(task Task) {
	crt := p.getCurrentTask()
	if crt == nil {
		return
	}

	if task.GetStatus() == SucceedStatus {
		p.idx += 1
	}
}

func (p *pipelineCoordinator) NextBatch(ctx context.Context) ([]Task, error) {
	crt := p.getCurrentTask()
	if crt == nil {
		return nil, nil
	}
	return []Task{crt}, nil
}

func (p *pipelineCoordinator) HandleFail(task Task, err error) FailOperation {
	if p.op == "" {
		return FailAndInterrupt
	}
	return p.op
}

func (p *pipelineCoordinator) getCurrentTask() Task {
	if p.idx >= len(p.tasks) {
		return nil
	}
	return p.tasks[p.idx]
}

var _ Coordinator = &pipelineCoordinator{}
