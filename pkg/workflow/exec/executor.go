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
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"sync"
	"time"
)

const (
	localExecName = "local"
)

func RegisterOperators(entryMgr dentry.Manager, cfg LocalConfig) error {
	jobrun.RegisterExecutorBuilder(localExecName, func(job *types.WorkflowJob) jobrun.Executor {
		return &localExecutor{
			job:      job,
			entryMgr: entryMgr,
			config:   cfg,
			results:  map[string]any{},
			logger:   logger.NewLogger("localExecutor").With(zap.String("job", job.Id)),
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
	results   map[string]any
	resultMux sync.Mutex
	logger    *zap.SugaredLogger
}

var _ jobrun.Executor = &localExecutor{}

func (b *localExecutor) Setup(ctx context.Context) (err error) {
	if !b.config.Workflow.Enable {
		return fmt.Errorf("workflow disabled")
	}

	startAt := time.Now()
	defer logOperationLatency(localExecName, "setup", startAt)

	// init workdir and copy entry file
	b.workdir, err = initWorkdir(ctx, b.config.Workflow.JobWorkdir, b.job)
	if err != nil {
		b.logger.Errorw("init job workdir failed", "err", err)
		return logOperationError(localExecName, "setup", err)
	}

	b.entryPath, err = entryWorkdirInit(ctx, b.job.Target.EntryID, b.entryMgr, b.workdir)
	if err != nil {
		b.logger.Errorw("copy target file to workdir failed", "err", err)
		return logOperationError(localExecName, "setup", err)
	}
	b.logger.Infow("job setup", "workdir", b.workdir, "entryPath", b.entryPath)

	return
}

func (b *localExecutor) DoOperation(ctx context.Context, step types.WorkflowJobStep) error {
	startAt := time.Now()
	defer logOperationLatency(localExecName, "do_operation", startAt)

	req := pluginapi.NewRequest()
	req.WorkPath = b.workdir
	req.EntryId = b.job.Target.EntryID
	req.EntryPath = b.entryPath

	req.Parameter = map[string]any{}
	b.resultMux.Lock()
	for k, v := range b.results {
		req.Parameter[k] = v
	}
	b.resultMux.Unlock()
	req.Parameter[pluginapi.ResEntryIdKey] = b.job.Target.EntryID
	req.Parameter[pluginapi.ResEntryPathKey] = b.entryPath
	req.Parameter[pluginapi.ResPluginName] = step.Plugin.PluginName
	req.Parameter[pluginapi.ResPluginVersion] = step.Plugin.Version
	req.Parameter[pluginapi.ResPluginType] = step.Plugin.PluginType
	req.Parameter[pluginapi.ResPluginAction] = step.Plugin.Action

	if step.Plugin.PluginType == types.TypeSource {
		info, err := plugin.SourceInfo(ctx, *step.Plugin)
		if err != nil {
			err = fmt.Errorf("get source info error: %s", err)
			return logOperationError(localExecName, "do_operation", err)
		}
		b.logger.Infow("running source plugin", "plugin", step.Plugin.PluginName, "source", info)
	}

	req.Action = step.Plugin.Action
	resp, err := plugin.Call(ctx, *step.Plugin, req)
	if err != nil {
		err = fmt.Errorf("plugin action error: %s", err)
		return logOperationError(localExecName, "do_operation", err)
	}
	if !resp.IsSucceed {
		err = fmt.Errorf("plugin action failed: %s", resp.Message)
		return logOperationError(localExecName, "do_operation", err)
	}
	if len(resp.Results) > 0 {
		b.resultMux.Lock()
		for k, v := range resp.Results {
			b.results[k] = v
		}
		b.resultMux.Unlock()
	}
	return nil
}

func (b *localExecutor) Collect(ctx context.Context) error {
	b.resultMux.Lock()
	rawManifests, needCollect := b.results[pluginapi.ResCollectManifests]
	b.resultMux.Unlock()
	if !needCollect {
		return nil
	}

	startAt := time.Now()
	defer logOperationLatency(localExecName, "collect", startAt)

	manifests, ok := rawManifests.([]pluginapi.CollectManifest)
	if !ok {
		msg := fmt.Sprintf("load collect manifest objects failed: unknown type %v", rawManifests)
		b.logger.Error(msg)
		return logOperationError(localExecName, "collect", fmt.Errorf(msg))
	}

	var errList []error
	for _, manifest := range manifests {
		for _, file := range manifest.NewFiles {
			if err := collectFile2BaseEntry(ctx, b.entryMgr, manifest.BaseEntry, b.workdir, file); err != nil {
				b.logger.Errorw("collect file to base entry failed", "entry", manifest.BaseEntry, "newFile", file.Name, "err", err)
				errList = append(errList, err)
			}
		}
	}
	if len(errList) > 0 {
		err := fmt.Errorf("collect file to base entry failed: %s, there are %d more similar errors", errList[0], len(errList))
		return logOperationError(localExecName, "collect", err)
	}
	return nil
}

func (b *localExecutor) Teardown(ctx context.Context) {
	startAt := time.Now()
	defer logOperationLatency(localExecName, "teardown", startAt)
	err := cleanupWorkdir(ctx, b.workdir)
	if err != nil {
		b.logger.Errorw("teardown failed: cleanup workdir error", "err", err)
		_ = logOperationError(localExecName, "teardown", err)
		return
	}
}

type LocalConfig struct {
	Workflow config.Workflow
}
