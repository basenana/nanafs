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
	"context"
	"errors"
	"github.com/basenana/nanafs/pkg/types"
	"sync"
)

var (
	ExecutorNotFound          = errors.New("executor not found")
	registeredExecutorBuilder []executorBuilder
)

func RegisterExecutorBuilder(name string, builder func(job *types.WorkflowJob) Executor) {
	registeredExecutorBuilder = append(registeredExecutorBuilder, executorBuilder{
		executor: name,
		build:    builder,
	})
}

func newExecutor(name string, job *types.WorkflowJob) (Executor, error) {
	for _, builder := range registeredExecutorBuilder {
		if builder.executor == name {
			return builder.build(job), nil
		}
	}
	return nil, ExecutorNotFound
}

type Executor interface {
	Setup(ctx context.Context) error
	DoOperation(ctx context.Context, step types.WorkflowJobStep) error
	Collect(ctx context.Context) error
	Teardown(ctx context.Context)
}

type Operator interface {
	Do(ctx context.Context, param *Parameter) error
}

type Parameter struct {
	FlowID  string
	Workdir string
	Result  *ResultData
}

type ResultData struct {
	Result   sync.Map
	TempData sync.Map
}

type executorBuilder struct {
	executor string
	build    func(job *types.WorkflowJob) Executor
}
