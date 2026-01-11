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
	"path"
	"time"

	"github.com/basenana/go-flow"
	"github.com/basenana/nanafs/pkg/cel"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/plugin"
	pluginapi "github.com/basenana/plugin/api"
	plugintypes "github.com/basenana/plugin/types"
	"go.uber.org/zap"
)

const (
	DefExecName = "default"
)

func newExecutor(ctrl *Controller, job *types.WorkflowJob) flow.Executor {
	return &defaultExecutor{
		job:       job,
		core:      ctrl.core,
		entry:     ctrl.store,
		store:     newPersistentStore(ctrl.store, job.Namespace),
		nfs:       newNamespacedFS(ctrl.core, ctrl.store, job.Namespace),
		pluginMgr: ctrl.pluginMgr,
		workdir:   path.Join(ctrl.workdir, fmt.Sprintf("job-%s", job.Id)),
		logger:    logger.NewLogger("defaultExecutor").With(zap.String("job", job.Id)),
	}
}

type defaultExecutor struct {
	job       *types.WorkflowJob
	core      core.Core
	entry     metastore.EntryStore
	store     pluginapi.PersistentStore
	nfs       pluginapi.NanaFS
	pluginMgr plugin.Manager
	workdir   string

	results Results
	logger  *zap.SugaredLogger
}

var _ flow.Executor = &defaultExecutor{}

func (b *defaultExecutor) Setup(ctx context.Context) (err error) {
	startAt := time.Now()
	defer logOperationLatency(DefExecName, "setup", startAt)

	if len(b.job.Targets.Entries) == 0 {
		return logOperationError(DefExecName, "setup", fmt.Errorf("no targets"))
	}

	targetURI := b.job.Targets.Entries[0]
	if len(b.job.Targets.Entries) > 1 {
		b.logger.Warn("multiple targets detected, using first one")
	}

	_, entry, err := b.core.GetEntryByPath(ctx, b.job.Namespace, targetURI)
	if err != nil {
		b.logger.Errorw("failed to get target entry by path", zap.Error(err))
		return logOperationError(DefExecName, "setup", err)
	}

	if entry == nil {
		b.logger.Errorw("target entry not found", zap.String("path", targetURI))
		return logOperationError(DefExecName, "setup", fmt.Errorf("target entry not found"))
	}

	// init workdir and copy entry file
	err = initWorkdir(ctx, b.workdir, b.job)
	if err != nil {
		b.logger.Errorw("init job workdir failed", "err", err)
		return logOperationError(DefExecName, "setup", err)
	}

	b.results = NewMemBasedResults()

	/*
		Setting trigger vars
	*/
	trigger := make(map[string]interface{})
	for k, v := range b.job.Parameters {
		trigger[k] = v
	}
	if !entry.IsGroup {
		enPath, err := entryWorkdirInit(ctx, b.job.Namespace, targetURI, b.core, b.workdir)
		if err != nil {
			b.logger.Errorw("copy target file to workdir failed", "err", err, "entry", targetURI)
			return logOperationError(DefExecName, "setup", err)
		}
		b.logger.Infow("copy entry to workdir", "entry", targetURI, "path", enPath)
		trigger["file_path"] = enPath
		trigger["entry_uri"] = targetURI
		trigger["parent_uri"] = path.Dir(targetURI)
	} else {
		trigger["parent_uri"] = targetURI
	}
	err = b.results.Set("trigger", trigger)
	if err != nil {
		b.logger.Errorw("set trigger failed", "err", err)
		return logOperationError(DefExecName, "setup", err)
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

	// Handle condition node
	if t.step.Type == "condition" {
		return b.execCondition(ctx, t)
	}

	// Handle switch node
	if t.step.Type == "switch" {
		return b.execSwitch(ctx, t)
	}

	// Handle matrix node
	if t.step.Matrix != nil {
		return b.execMatrix(ctx, t)
	}

	resp, err := b.callPlugin(ctx, t.step, nil)
	if err != nil {
		return err
	}
	if err = b.results.Set(t.step.Name, resp); err != nil {
		return logOperationError(DefExecName, "update_context", err)
	}
	return nil
}

func (b *defaultExecutor) callPlugin(ctx context.Context, step *types.WorkflowJobNode, patchData map[string]any) (map[string]any, error) {
	resultData := b.results.Data()
	if len(patchData) > 0 {
		for k, v := range patchData {
			resultData[k] = v
		}
	}
	req := b.newPluginRequest(step, resultData)
	resp, err := callPlugin(ctx, b.job, step, b.pluginMgr, req, b.workdir)
	if err != nil {
		return nil, logOperationError(DefExecName, "call_plugin", err)
	}

	return resp.Results, nil
}

func (b *defaultExecutor) execCondition(ctx context.Context, t *Task) error {
	condition := t.step.Condition
	if condition == "" {
		return fmt.Errorf("condition node missing expression")
	}

	resultData := b.results.Data()
	matched, err := cel.EvalCELWithVars(resultData, condition)
	if err != nil {
		return fmt.Errorf("evaluate condition failed: %w", err)
	}

	if matched {
		if next, ok := t.step.Branches["true"]; ok {
			t.SetBranchNext(next)
		}
	} else {
		if next, ok := t.step.Branches["false"]; ok {
			t.SetBranchNext(next)
		}
	}

	return nil
}

func (b *defaultExecutor) execSwitch(ctx context.Context, t *Task) error {
	field := t.step.Params["field"]
	if field == "" {
		return fmt.Errorf("switch node missing field parameter")
	}

	// Get field value from context results
	resultData := b.results.Data()
	fieldValue, ok := resultData[field]
	if !ok {
		if t.step.Default != "" {
			t.SetBranchNext(t.step.Default)
			return nil
		}
		return fmt.Errorf("switch node field '%s' not found in context", field)
	}

	strValue := fmt.Sprintf("%v", fieldValue)
	for _, c := range t.step.Cases {
		if c.Value == strValue {
			t.SetBranchNext(c.Next)
			return nil
		}
	}

	if t.step.Default != "" {
		t.SetBranchNext(t.step.Default)
	}

	return nil
}

func (b *defaultExecutor) execMatrix(ctx context.Context, t *Task) error {
	matrix := t.step.Matrix
	if matrix == nil || len(matrix.Data) == 0 {
		return fmt.Errorf("matrix node has no matrix configuration")
	}

	// Get iteration data from context
	iterations, err := renderMatrixData(matrix.Data, b.results.Data())
	if err != nil {
		return fmt.Errorf("failed to render matrix data: %w", err)
	}

	if len(iterations) == 0 {
		b.logger.Infow("matrix has no iterations, skipping")
		return nil
	}

	b.logger.Infow("executing matrix", "iterations", len(iterations))

	// Execute iterations sequentially
	results, err := b.execMatrixSequential(ctx, t, iterations)
	if err != nil {
		return err
	}

	if err = b.results.Set(t.step.Name, map[string]any{"matrix_results": results}); err != nil {
		return logOperationError(DefExecName, "matrix_store_results", err)
	}

	b.logger.Infow("matrix execution completed", "total_iterations", len(iterations))
	return nil
}

func (b *defaultExecutor) execMatrixSequential(ctx context.Context, t *Task, iterations []matrixIteration) ([]map[string]any, error) {
	var results []map[string]any

	for i, iteration := range iterations {
		iterationResult, err := b.callPlugin(ctx, t.step, map[string]any{"matrix": iteration.Variables})
		if err != nil {
			return nil, err
		}

		results = append(results, iterationResult)
		b.logger.Infow("matrix iteration completed", "iteration", i+1, "total", len(iterations))
	}

	return results, nil
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

func (b *defaultExecutor) newPluginRequest(step *types.WorkflowJobNode, resultData map[string]any) *pluginapi.Request {
	req := pluginapi.NewRequest()
	req.Store = b.store
	req.FS = b.nfs

	globalVars := injectGlobalVars(b.job)
	for k, v := range globalVars {
		resultData[k] = v
	}

	req.Parameter = make(map[string]any)
	for k, v := range step.Input {
		req.Parameter[k] = renderParams(v, resultData)
	}

	return req
}

func callPlugin(ctx context.Context, job *types.WorkflowJob, step *types.WorkflowJobNode, mgr plugin.Manager, req *pluginapi.Request, workdir string) (*pluginapi.Response, error) {
	pc := plugintypes.PluginCall{
		JobID:       job.Id,
		Workflow:    job.Workflow,
		Namespace:   job.Namespace,
		WorkingPath: workdir,
		PluginName:  step.Type,
		Params:      step.Params,
	}
	resp, err := mgr.Call(ctx, pc, req)
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
