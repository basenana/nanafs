/*
 Copyright 2023 NanaFS Authors.

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
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

const (
	localExecName = "local"
)

func RegisterOperators(entryMgr dentry.Manager, cfg LocalConfig) error {
	jobrun.RegisterExecutorBuilder(localExecName, func(job *types.WorkflowJob) jobrun.Executor {
		return &localExecutor{
			job: job, entryMgr: entryMgr, config: cfg,
			logger: logger.NewLogger("localExecutor").With(zap.String("job", job.Id)),
		}
	})
	return nil
}

type localExecutor struct {
	job       *types.WorkflowJob
	workdir   string
	entryPath string
	entryMgr  dentry.Manager
	config    LocalConfig
	logger    *zap.SugaredLogger
}

var _ jobrun.Executor = &localExecutor{}

func (b *localExecutor) Setup(ctx context.Context) (err error) {
	if !b.config.Workflow.Enable {
		return fmt.Errorf("workflow disabled")
	}

	// init workdir and copy entry file
	b.workdir, err = initWorkdir(ctx, b.config.Workflow.JobWorkdir, b.job)
	if err != nil {
		b.logger.Errorw("init job workdir failed", "err", err)
		return
	}

	b.entryPath, err = entryWorkdirInit(ctx, b.job.Target.EntryID, b.entryMgr, b.workdir)
	if err != nil {
		b.logger.Errorw("copy target file to workdir failed", "err", err)
		return
	}
	b.logger.Infow("job setup", "workdir", b.workdir, "entryPath", b.entryPath)
	return
}

func (b *localExecutor) DoOperation(ctx context.Context, step types.WorkflowJobStep) error {
	req := stub.NewRequest()
	req.WorkPath = b.workdir
	req.EntryId = b.job.Target.EntryID
	req.EntryPath = b.entryPath

	req.Action = step.Plugin.PluginName
	req.Parameter = step.Plugin.Parameters
	resp, err := plugin.Call(ctx, *step.Plugin, req)
	if err != nil {
		return fmt.Errorf("plugin action error: %s", err)
	}
	if !resp.IsSucceed {
		return fmt.Errorf("plugin action failed: %s", resp.Message)
	}
	return nil
}

func (b *localExecutor) Collect(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (b *localExecutor) Teardown(ctx context.Context) {
	err := cleanupWorkdir(ctx, b.workdir)
	if err != nil {
		b.logger.Errorw("teardown failed: cleanup workdir error", "err", err)
		return
	}
}

type LocalConfig struct {
	Workflow config.Workflow
}