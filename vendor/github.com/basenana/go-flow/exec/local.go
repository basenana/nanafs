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

package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/basenana/go-flow/cfg"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/utils"
)

const (
	ShellOperator  = "shell"
	PythonOperator = "python"
)

func NewLocalOperatorBuilderRegister() *Registry {
	r := NewPipeOperatorBuilderRegister()
	_ = r.Register(ShellOperator, newLocalShellOperator)
	_ = r.Register(PythonOperator, newLocalPythonOperator)

	return r
}

type LocalExecutor struct {
	flow     *flow.Flow
	registry *Registry
	logger   utils.Logger
}

func (l *LocalExecutor) Setup(ctx context.Context) error {
	if err := initFlowWorkDir(cfg.LocalWorkdirBase, l.flow.ID); err != nil {
		l.logger.Errorf("init work dir failed: %s", err)
		return err
	}
	return nil
}

func (l *LocalExecutor) Teardown(ctx context.Context) {
	if err := cleanUpFlowWorkDir(cfg.LocalWorkdirBase, l.flow.ID); err != nil {
		l.logger.Errorf("teardown failed: %s", err)
		return
	}
}

func (l *LocalExecutor) DoOperation(ctx context.Context, task flow.Task, operatorSpec flow.Spec) error {
	builder, err := l.registry.FindBuilder(operatorSpec.Type)
	if err != nil {
		return err
	}
	operator, err := builder(task, operatorSpec)
	if err != nil {
		l.logger.Errorf("build operator %s failed: %s", operatorSpec.Type, err)
		return err
	}

	param := &flow.Parameter{
		FlowID:  l.flow.ID,
		Workdir: flowWorkdir(cfg.LocalWorkdirBase, l.flow.ID),
		Result:  &flow.ResultData{},
	}
	err = operator.Do(ctx, param)
	if err != nil {
		l.logger.Errorf("run operator %s failed: %s", operatorSpec.Type, err)
		return err
	}
	return nil
}

func NewLocalExecutor(flow *flow.Flow, registry *Registry) flow.Executor {
	return &LocalExecutor{
		flow:     flow,
		registry: registry,
		logger:   utils.NewLogger("local").With(flow.ID),
	}
}

type localShellOperator struct {
	command string
	spec    flow.Spec
}

func (l *localShellOperator) Do(ctx context.Context, param *flow.Parameter) error {
	command := l.command
	if command == "" {
		command = "sh"
	}

	shellFile := fmt.Sprintf("script_%d.sh", time.Now().Unix())
	shellFilePath := path.Join(param.Workdir, shellFile)

	if l.spec.Script.Content != "" {
		shF, err := os.Create(shellFilePath)
		if err != nil {
			return err
		}

		switch command {
		case "sh":
			if _, err = shF.WriteString("#!/bin/sh\nset -xe\n"); err != nil {
				return err
			}
		}
		if _, err = shF.WriteString(l.spec.Script.Content); err != nil {
			return err
		}

		err = os.Chmod(shF.Name(), 0755)
		if err != nil {
			return err
		}
	}

	var (
		stdout bytes.Buffer
		cmd    *exec.Cmd
	)
	if l.spec.Script != nil && len(l.spec.Script.Command) > 0 {
		args := l.spec.Script.Command[1:]
		cmd = exec.Command(l.spec.Script.Command[0], args...)
	} else {
		cmd = exec.Command(command, shellFilePath)
	}

	env := os.Environ()
	for k, v := range l.spec.Script.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env
	cmd.Dir = param.Workdir
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func newLocalShellOperator(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error) {
	if operatorSpec.Script == nil {
		return nil, fmt.Errorf("shell is nil")
	}
	return &localShellOperator{spec: operatorSpec}, nil
}

type localPythonOperator struct {
	spec flow.Spec
}

func (l *localPythonOperator) Do(ctx context.Context, param *flow.Parameter) error {
	pythonBin := "python"
	if cfg.LocalPythonVersion == "3" {
		pythonBin = "python3"
	}
	op := localShellOperator{spec: l.spec, command: pythonBin}
	return op.Do(ctx, param)
}

func newLocalPythonOperator(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error) {
	return &localPythonOperator{spec: operatorSpec}, nil
}
