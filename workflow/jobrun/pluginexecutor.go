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

package jobrun

import (
	"context"
	"errors"
	"fmt"
	"github.com/basenana/go-flow"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"path"
	"time"
)

const (
	DataPipeExecName = "pipe"
	FileExecName     = "file"
)

func newExecutor(ctrl *Controller, job *types.WorkflowJob) flow.Executor {
	switch job.QueueName {
	case types.WorkflowQueuePipe:
		return &pipeExecutor{
			job:       job,
			pluginMgr: ctrl.pluginMgr,
			core:      ctrl.core,
			store:     ctrl.store,
			logger:    logger.NewLogger("pipeExecutor").With(zap.String("job", job.Id)),
		}
	default:
		return &fileExecutor{
			job:       job,
			pluginMgr: ctrl.pluginMgr,
			core:      ctrl.core,
			store:     ctrl.store,
			workdir:   path.Join(ctrl.workdir, fmt.Sprintf("job-%s", job.Id)),
			logger:    logger.NewLogger("fileExecutor").With(zap.String("job", job.Id)),
		}
	}
}

type pipeExecutor struct {
	job       *types.WorkflowJob
	pluginMgr *plugin.Manager
	core      core.Core
	store     metastore.EntryStore
	logger    *zap.SugaredLogger

	ctxResults pluginapi.Results
	targets    []*types.Entry
}

var _ flow.Executor = &pipeExecutor{}

func (p *pipeExecutor) Setup(ctx context.Context) error {
	var (
		en  *types.Entry
		err error
	)

	p.ctxResults = pluginapi.NewMemBasedResults()
	for _, eid := range p.job.Target.Entries {
		en, err = p.core.GetEntry(ctx, p.job.Namespace, eid)
		if err != nil && !errors.Is(err, types.ErrNotFound) {
			return fmt.Errorf("get entry by id failed %w", err)
		}
		p.targets = append(p.targets, en)
	}

	return nil
}

func (p *pipeExecutor) Exec(ctx context.Context, flow *flow.Flow, task flow.Task) (err error) {
	t, ok := task.(*Task)
	if !ok {
		return fmt.Errorf("not job task")
	}

	startAt := time.Now()
	defer logOperationLatency(DataPipeExecName, "do_operation", startAt)

	defer func() {
		if panicErr := utils.Recover(); panicErr != nil {
			p.logger.Errorw("executor panic", "err", panicErr)
			err = panicErr
		}
	}()
	req := newPluginRequest(p.job, t.step, p.ctxResults, p.targets...)
	var resp *pluginapi.Response
	resp, err = callPlugin(ctx, p.job, *t.step.Plugin, p.pluginMgr, p.store, req, p.logger)
	if err != nil {
		return logOperationError(DataPipeExecName, "call_plugin", err)
	}

	for k, v := range resp.Results {
		if err = p.ctxResults.Set(k, v); err != nil {
			return logOperationError(DataPipeExecName, "update_context", err)
		}
	}
	return
}

func (p *pipeExecutor) Teardown(ctx context.Context) error {
	return nil
}

type fileExecutor struct {
	job       *types.WorkflowJob
	core      core.Core
	store     metastore.EntryStore
	pluginMgr *plugin.Manager

	workdir    string
	entryPath  string
	entryURI   string
	cachedData *pluginapi.CachedData
	targets    []*types.Entry

	ctxResults pluginapi.Results
	logger     *zap.SugaredLogger
}

var _ flow.Executor = &fileExecutor{}

func (b *fileExecutor) Setup(ctx context.Context) (err error) {
	startAt := time.Now()
	defer logOperationLatency(FileExecName, "setup", startAt)

	// init workdir and copy entry file
	err = initWorkdir(ctx, b.workdir, b.job)
	if err != nil {
		b.logger.Errorw("init job workdir failed", "err", err)
		return logOperationError(FileExecName, "setup", err)
	}

	b.ctxResults, err = pluginapi.NewFileBasedResults(pluginapi.ResultFilePath(b.workdir))
	if err != nil {
		b.logger.Errorw("init job ctx result failed", "err", err)
		return logOperationError(FileExecName, "setup", err)
	}

	for _, enID := range b.job.Target.Entries {
		en, err := b.core.GetEntry(ctx, b.job.Namespace, enID)
		if err != nil && !errors.Is(err, types.ErrNotFound) {
			return fmt.Errorf("get entry by id failed %w", err)
		}
		b.targets = append(b.targets, en)

		epath, err := entryWorkdirInit(ctx, b.job.Namespace, enID, b.core, b.workdir)
		if err != nil {
			b.logger.Errorw("copy target file to workdir failed", "err", err, "entry", enID)
			return logOperationError(FileExecName, "setup", err)
		}
		b.logger.Infow("copy entry to workdir", "entry", enID, "path", epath)
	}

	if b.job.Target.ParentEntryID != 0 && len(b.job.Target.Entries) == 0 {
		// base on parent entry
		b.cachedData, err = initParentDirCacheData(ctx, b.job.Namespace, b.core, b.job.Target.ParentEntryID)
		if err != nil {
			b.logger.Errorw("build parent cache data failed", "parent", b.job.Target.ParentEntryID, "err", err)
			return logOperationError(FileExecName, "setup", err)
		}
	}
	b.logger.Infow("job setup finish", "workdir", b.workdir, "entryPath", b.entryPath)

	return
}

func (b *fileExecutor) Exec(ctx context.Context, flow *flow.Flow, task flow.Task) (err error) {
	t, ok := task.(*Task)
	if !ok {
		return fmt.Errorf("not job task")
	}

	startAt := time.Now()
	defer logOperationLatency(FileExecName, "do_operation", startAt)

	defer func() {
		if panicErr := utils.Recover(); panicErr != nil {
			b.logger.Errorw("executor panic", "err", panicErr)
			err = panicErr
		}
	}()

	req := newPluginRequest(b.job, t.step, b.ctxResults, b.targets...)
	req.WorkPath = b.workdir
	req.CacheData = b.cachedData
	req.ContextResults = b.ctxResults

	var resp *pluginapi.Response
	resp, err = callPlugin(ctx, b.job, *t.step.Plugin, b.pluginMgr, b.store, req, b.logger)
	if err != nil {
		return logOperationError(FileExecName, "call_plugin", err)
	}

	for k, v := range resp.Results {
		if err = b.ctxResults.Set(k, v); err != nil {
			return logOperationError(FileExecName, "update_context", err)
		}
	}

	err = b.tryCollect(ctx, resp)
	if err != nil {
		return logOperationError(FileExecName, "collect_data", err)
	}
	return
}

func (b *fileExecutor) tryCollect(ctx context.Context, resp *pluginapi.Response) error {
	startAt := time.Now()
	defer logOperationLatency(FileExecName, "collect", startAt)
	if len(resp.NewEntries) > 0 {
		// collect files
		err := b.collectEntries(ctx, resp.NewEntries)
		if err != nil {
			return err
		}
	}

	if b.cachedData != nil && b.cachedData.NeedReCache() {
		b.logger.Infow("collect cache data")
		if err := writeParentDirCacheData(ctx, b.job.Namespace, b.core, b.job.Target.ParentEntryID, b.cachedData); err != nil {
			b.logger.Errorw("write parent cached data back failed", "err", err)
			return err
		}
	}

	return nil
}

func (b *fileExecutor) collectEntries(ctx context.Context, manifests []pluginapi.CollectManifest) error {
	b.logger.Infow("collect files", "manifests", len(manifests))
	var (
		errList []error
		en      *types.Entry
		err     error
	)
	for _, manifest := range manifests {
		for i := range manifest.NewFiles {
			file := &(manifest.NewFiles[i])
			if en, err = collectFile2BaseEntry(ctx, b.job.Namespace, b.core, manifest.BaseEntry, b.workdir, file); err != nil {
				b.logger.Errorw("collect file to base entry failed", "entry", manifest.BaseEntry, "newFile", file.Name, "err", err)
				errList = append(errList, err)
				continue
			}

			if file.Document != nil {
				if en == nil {
					errList = append(errList, fmt.Errorf("collect document %s error: entry id is empty", file.Document.Title))
					continue
				}
				b.logger.Infow("collect documents", "entryId", file.ID)
				if err = collectFile2Document(ctx, en, file.Document); err != nil {
					return logOperationError(FileExecName, "collect", err)
				}
			}
		}
	}
	if len(errList) > 0 {
		err := fmt.Errorf("collect file to base entry failed: %s, there are %d more similar errors", errList[0], len(errList))
		return logOperationError(FileExecName, "collect", err)
	}
	return nil
}

func (b *fileExecutor) Teardown(ctx context.Context) error {
	startAt := time.Now()
	defer logOperationLatency(FileExecName, "teardown", startAt)
	err := cleanupWorkdir(ctx, b.workdir)
	if err != nil {
		b.logger.Errorw("teardown failed: cleanup workdir error", "err", err)
		_ = logOperationError(FileExecName, "teardown", err)
		return err
	}
	return nil
}

func callPlugin(ctx context.Context, job *types.WorkflowJob, ps types.PlugScope, mgr *plugin.Manager,
	store metastore.EntryStore, req *pluginapi.Request, logger *zap.SugaredLogger) (*pluginapi.Response, error) {
	req.Action = ps.Action
	resp, err := mgr.Call(ctx, job, ps, req)
	if err != nil {
		err = fmt.Errorf("plugin action error: %s", err)
		return nil, err
	}
	if !resp.IsSucceed {
		err = fmt.Errorf("plugin action failed: %s", resp.Message)
		return nil, err
	}
	return resp, nil
}
