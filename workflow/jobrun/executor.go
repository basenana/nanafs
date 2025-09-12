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
	"sort"
	"time"
)

const (
	DefExecName = "default"
)

func newExecutor(ctrl *Controller, job *types.WorkflowJob) flow.Executor {
	return &defaultExecutor{
		job:       job,
		core:      ctrl.core,
		entry:     ctrl.store,
		store:     ctrl.store,
		pluginMgr: ctrl.pluginMgr,
		workdir:   path.Join(ctrl.workdir, fmt.Sprintf("job-%s", job.Id)),
		logger:    logger.NewLogger("defaultExecutor").With(zap.String("job", job.Id)),
	}
}

type defaultExecutor struct {
	job       *types.WorkflowJob
	core      core.Core
	entry     metastore.EntryStore
	store     pluginapi.ContextStore
	pluginMgr *plugin.Manager
	workdir   string

	entries    []*pluginapi.Entry
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
		b.entries = append(b.entries, en)
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
		req  = newPluginRequest(b.workdir, b.job, t.step, b.store, b.ctxResults, b.entries...)
		resp *pluginapi.Response
	)
	resp, err = callPlugin(ctx, t.job, t.step, b.pluginMgr, req)
	if err != nil {
		return logOperationError(DefExecName, "call_plugin", err)
	}

	for k, v := range resp.Results {
		if err = b.ctxResults.Set(k, v); err != nil {
			return logOperationError(DefExecName, "update_context", err)
		}
	}

	err = b.tryUpdateEntries(ctx, resp)
	if err != nil {
		return logOperationError(DefExecName, "collect_data", err)
	}
	return
}

func (b *defaultExecutor) tryUpdateEntries(ctx context.Context, resp *pluginapi.Response) error {
	if len(resp.ModifyEntries) == 0 {
		return nil
	}

	var (
		latest     []*pluginapi.Entry
		allEntries = make(map[string]*pluginapi.Entry)
	)
	for _, en := range b.entries {
		allEntries[fmt.Sprintf("%d/%s", en.Parent, en.Name)] = en
	}
	for i := range resp.ModifyEntries {
		en := resp.ModifyEntries[i]
		en.Dirty = true
		allEntries[fmt.Sprintf("%d/%s", en.Parent, en.Name)] = &en
	}

	for _, en := range allEntries {
		latest = append(latest, en)
	}

	sort.Slice(latest, func(i, j int) bool {
		if latest[i].Parent != latest[j].Parent {
			return latest[i].Parent < latest[j].Parent
		}
		return latest[i].Name < latest[j].Name
	})

	b.entries = latest

	return nil
}

func (b *defaultExecutor) collectEntries(ctx context.Context) error {
	var (
		entries     = b.entries
		needCollect = false
		errList     []error
		err         error
	)
	for _, en := range b.entries {
		if en.Dirty {
			needCollect = true
			break
		}
	}
	if !needCollect {
		return nil
	}

	startAt := time.Now()
	defer logOperationLatency(DefExecName, "collect_entries", startAt)
	b.logger.Infow("collect files", "entries", len(entries))
	for _, en := range entries {
		if err = collectAndModifyEntry(ctx, b.job.Namespace, b.core, b.workdir, en); err != nil {
			b.logger.Errorw("collect file to base entry failed", "parent", en.Parent, "name", en.Name, "newFile", "err", err)
			errList = append(errList, err)
			continue
		}
	}
	if len(errList) > 0 {
		err = fmt.Errorf("collect file to base entry failed: %s, there are %d more similar errors", errList[0], len(errList))
		return logOperationError(DefExecName, "collect", err)
	}
	return nil
}

func (b *defaultExecutor) Teardown(ctx context.Context) error {
	startAt := time.Now()
	defer logOperationLatency(DefExecName, "teardown", startAt)

	err := b.collectEntries(ctx)
	if err != nil {
		return err
	}

	err = cleanupWorkdir(ctx, b.workdir)
	if err != nil {
		b.logger.Errorw("teardown failed: cleanup workdir error", "err", err)
		_ = logOperationError(DefExecName, "teardown", err)
		return err
	}
	return nil
}

func newPluginRequest(workingPath string, job *types.WorkflowJob, step *types.WorkflowJobNode, store pluginapi.ContextStore, result pluginapi.Results, entries ...*pluginapi.Entry) *pluginapi.Request {
	req := pluginapi.NewRequest()
	req.JobID = job.Id
	req.WorkingPath = workingPath
	req.Namespace = job.Namespace
	req.PluginName = step.Type
	req.ContextStore = store

	req.Parameter = map[string]string{}
	resultData := result.Data()
	for k, v := range step.Parameters {
		req.Parameter[k] = renderParams(v, resultData)
	}

	for _, en := range entries {
		if en.IsGroup && req.GroupEntryId == 0 {
			req.GroupEntryId = en.ID
		}
		req.Entries = append(req.Entries, *en)
	}
	return req
}

func callPlugin(ctx context.Context, job *types.WorkflowJob, step *types.WorkflowJobNode, mgr *plugin.Manager,
	req *pluginapi.Request) (*pluginapi.Response, error) {
	pc := types.PluginCall{PluginName: step.Type}
	resp, err := mgr.Call(ctx, job, pc, req)
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
