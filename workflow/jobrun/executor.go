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
	DefExecName = "default"
)

func newExecutor(ctrl *Controller, job *types.WorkflowJob) flow.Executor {
	return &defaultExecutor{
		job:       job,
		pluginMgr: ctrl.pluginMgr,
		core:      ctrl.core,
		store:     ctrl.store,
		workdir:   path.Join(ctrl.workdir, fmt.Sprintf("job-%s", job.Id)),
		logger:    logger.NewLogger("defaultExecutor").With(zap.String("job", job.Id)),
	}
}

type defaultExecutor struct {
	job       *types.WorkflowJob
	core      core.Core
	store     metastore.EntryStore
	pluginMgr *plugin.Manager

	workdir string
	Entries []*pluginapi.Entry

	ctxResults pluginapi.Results
	logger     *zap.SugaredLogger
}

var _ flow.Executor = &defaultExecutor{}

func (b *defaultExecutor) Setup(ctx context.Context) (err error) {
	startAt := time.Now()
	defer logOperationLatency(DefExecName, "setup", startAt)

	// init workdir and copy entry file
	err = initWorkdir(ctx, b.workdir, b.job)
	if err != nil {
		b.logger.Errorw("init job workdir failed", "err", err)
		return logOperationError(DefExecName, "setup", err)
	}

	b.ctxResults, err = pluginapi.NewFileBasedResults(pluginapi.ResultFilePath(b.workdir))
	if err != nil {
		b.logger.Errorw("init job ctx result failed", "err", err)
		return logOperationError(DefExecName, "setup", err)
	}

	for _, enUri := range b.job.Targets.Entries {
		en, err := entryWorkdirInit(ctx, b.job.Namespace, enUri, b.core, b.workdir)
		if err != nil {
			b.logger.Errorw("copy target file to workdir failed", "err", err, "entry", enUri)
			return logOperationError(DefExecName, "setup", err)
		}
		b.Entries = append(b.Entries, en)
		b.logger.Infow("copy entry to workdir", "entry", enUri)
	}

	b.logger.Infow("job setup finish", "workdir", b.workdir)

	return
}

func (b *defaultExecutor) Exec(ctx context.Context, flow *flow.Flow, task flow.Task) (err error) {
	t, ok := task.(*Task)
	if !ok {
		return fmt.Errorf("not job task")
	}

	startAt := time.Now()
	defer logOperationLatency(DefExecName, "do_operation", startAt)

	defer func() {
		if panicErr := utils.Recover(); panicErr != nil {
			b.logger.Errorw("executor panic", "err", panicErr)
			err = panicErr
		}
	}()

	var (
		req  = newPluginRequest(b.workdir, b.job, t.step, b.ctxResults, b.Entries...)
		resp *pluginapi.Response
	)
	resp, err = callPlugin(ctx, b.job, *t.step.Plugin, b.pluginMgr, req, b.logger)
	if err != nil {
		return logOperationError(DefExecName, "call_plugin", err)
	}

	for k, v := range resp.Results {
		if err = b.ctxResults.Set(k, v); err != nil {
			return logOperationError(DefExecName, "update_context", err)
		}
	}

	for _, en := range resp.NewEntries {
		_ = en
	}

	err = b.tryCollect(ctx, resp)
	if err != nil {
		return logOperationError(DefExecName, "collect_data", err)
	}
	return
}

func (b *defaultExecutor) tryCollect(ctx context.Context, resp *pluginapi.Response) error {
	startAt := time.Now()
	defer logOperationLatency(DefExecName, "collect", startAt)
	if len(resp.NewEntries) > 0 {
		// collect files
		err := b.collectEntries(ctx, resp.NewEntries)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *defaultExecutor) collectEntries(ctx context.Context, manifests []pluginapi.CollectManifest) error {
	b.logger.Infow("collect files", "manifests", len(manifests))
	var (
		errList []error
		en      *types.Entry
		err     error
	)
	for _, manifest := range manifests {
		for i := range manifest.NewFiles {
			file := &(manifest.NewFiles[i])
			if en, err = collectFile2BaseEntry(ctx, b.job.Namespace, b.core, manifest.ParentEntry, b.workdir, file); err != nil {
				b.logger.Errorw("collect file to base entry failed", "entry", manifest.ParentEntry, "newFile", file.Name, "err", err)
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
					return logOperationError(DefExecName, "collect", err)
				}
			}
		}
	}
	if len(errList) > 0 {
		err := fmt.Errorf("collect file to base entry failed: %s, there are %d more similar errors", errList[0], len(errList))
		return logOperationError(DefExecName, "collect", err)
	}
	return nil
}

func (b *defaultExecutor) Teardown(ctx context.Context) error {
	startAt := time.Now()
	defer logOperationLatency(DefExecName, "teardown", startAt)
	err := cleanupWorkdir(ctx, b.workdir)
	if err != nil {
		b.logger.Errorw("teardown failed: cleanup workdir error", "err", err)
		_ = logOperationError(DefExecName, "teardown", err)
		return err
	}
	return nil
}

func newPluginRequest(workingPath string, job *types.WorkflowJob, step *types.WorkflowJobNode, result pluginapi.Results, entries ...*pluginapi.Entry) *pluginapi.Request {
	req := pluginapi.NewRequest()
	req.WorkingPath = workingPath
	req.Namespace = job.Namespace
	req.PluginName = step.Type

	req.Parameter = map[string]string{}
	for k, v := range step.Parameters {
		// TODO render parameter using result
		req.Parameter[k] = v
	}

	for _, en := range entries {
		req.Entries = append(req.Entries, *en)
	}
	return req
}

func callPlugin(ctx context.Context, job *types.WorkflowJob, pcall types.PluginCall, mgr *plugin.Manager,
	req *pluginapi.Request, logger *zap.SugaredLogger) (*pluginapi.Response, error) {
	resp, err := mgr.Call(ctx, job, pcall, req)
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
