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
)

type Task interface {
	GetName() string
	GetStatus() string
	SetStatus(string)
	GetMessage() string
	SetMessage(string)
}

type Executor interface {
	Setup(ctx context.Context) error
	Exec(ctx context.Context, flow *Flow, task Task) error
	Teardown(ctx context.Context) error
}

type BasicTask struct {
	Name    string
	Status  string
	Message string
}

var _ Task = &BasicTask{}

func (t *BasicTask) GetName() string {
	return t.Name
}

func (t *BasicTask) GetStatus() string {
	return t.Status
}

func (t *BasicTask) SetStatus(status string) {
	t.Status = status
}

func (t *BasicTask) GetMessage() string {
	return t.Message
}

func (t *BasicTask) SetMessage(msg string) {
	t.Message = msg
}

type FunctionTask struct {
	*BasicTask
	runFn func(ctx context.Context) error
}

func (f *FunctionTask) Run(ctx context.Context) error {
	return f.runFn(ctx)
}

func NewFuncTask(name string, runFn func(ctx context.Context) error) Task {
	return &FunctionTask{
		BasicTask: &BasicTask{
			Name:   name,
			Status: InitializingStatus,
		},
		runFn: runFn,
	}
}

type Runnable interface {
	Run(ctx context.Context) error
}

type functionExecutor struct{}

var _ Executor = &functionExecutor{}

func (s *functionExecutor) Exec(ctx context.Context, flow *Flow, task Task) error {
	t, ok := task.(Runnable)
	if !ok {
		return fmt.Errorf("not a function")
	}
	return t.Run(ctx)
}

func (s *functionExecutor) Setup(ctx context.Context) error {
	return nil
}

func (s *functionExecutor) Teardown(ctx context.Context) error {
	return nil
}
