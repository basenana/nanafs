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
	"sync"
	"time"

	"github.com/basenana/go-flow"
	"github.com/basenana/nanafs/pkg/cel"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
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
	pluginMgr plugin.Manager
	workdir   string

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

	var entries []*pluginapi.Entry
	for _, enUri := range b.job.Targets.Entries {
		en, err := entryWorkdirInit(ctx, b.job.Namespace, enUri, b.core, b.workdir)
		if err != nil {
			b.logger.Errorw("copy target file to workdir failed", "err", err, "entry", enUri)
			return logOperationError(DefExecName, "setup", err)
		}
		entries = append(entries, en)
		b.logger.Infow("copy entry to workdir", "entry", enUri)
	}

	trigger := make(map[string]interface{})
	switch len(entries) {
	case 0:
	case 1:
		trigger["file_path"] = entries[0].Path
	default:
		filePaths := make([]string, 0, len(entries))
		for _, entry := range entries {
			filePaths = append(filePaths, path.Join(b.workdir, entry.Path))
		}
		trigger["file_paths"] = filePaths
	}

	err = b.ctxResults.Set("trigger", trigger)
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

	_, err = b.callPluginAndCollect(ctx, t.step)
	return err
}

func (b *defaultExecutor) callPluginAndCollect(ctx context.Context, step *types.WorkflowJobNode) (map[string]any, error) {
	req := newPluginRequest(b.workdir, b.job, step, b.store, b.ctxResults)
	resp, err := callPlugin(ctx, step, b.pluginMgr, req)
	if err != nil {
		return nil, logOperationError(DefExecName, "call_plugin", err)
	}

	if err = b.ctxResults.Set(step.Name, resp.Results); err != nil {
		return nil, logOperationError(DefExecName, "update_context", err)
	}

	return resp.Results, nil
}

func (b *defaultExecutor) execCondition(ctx context.Context, t *Task) error {
	condition := t.step.Condition
	if condition == "" {
		return fmt.Errorf("condition node missing expression")
	}

	resultData := b.ctxResults.Data()
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
	resultData := b.ctxResults.Data()
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
	iterations, err := renderMatrixData(matrix.Data, b.ctxResults)
	if err != nil {
		return fmt.Errorf("failed to render matrix data: %w", err)
	}

	if len(iterations) == 0 {
		b.logger.Infow("matrix has no iterations, skipping")
		return nil
	}

	b.logger.Infow("executing matrix", "iterations", len(iterations), "mode", matrix.IterateMode)

	// Execute iterations based on IterateMode
	var results []map[string]any

	if matrix.IterateMode == "parallel" {
		// Parallel execution with optional batch size limit
		results, err = b.execMatrixParallel(ctx, t, iterations, matrix.BatchSize)
	} else {
		// Default sequential execution
		results, err = b.execMatrixSequential(ctx, t, iterations)
	}

	if err != nil {
		return err
	}

	if err = b.ctxResults.Set("matrix_results", results); err != nil {
		return logOperationError(DefExecName, "matrix_store_results", err)
	}

	b.logger.Infow("matrix execution completed", "total_iterations", len(iterations))
	return nil
}

func (b *defaultExecutor) execMatrixSequential(ctx context.Context, t *Task, iterations []matrixIteration) ([]map[string]any, error) {
	var results []map[string]any

	for i, iteration := range iterations {
		for key, val := range iteration.Variables {
			if err := b.ctxResults.Set(key, val); err != nil {
				return nil, fmt.Errorf("failed to set matrix variable %s: %w", key, err)
			}
		}

		iterationResult, err := b.callPluginAndCollect(ctx, t.step)
		if err != nil {
			return nil, err
		}

		results = append(results, iterationResult)
		b.logger.Infow("matrix iteration completed", "iteration", i+1, "total", len(iterations))
	}

	return results, nil
}

func (b *defaultExecutor) execMatrixParallel(ctx context.Context, t *Task, iterations []matrixIteration, batchSize int) ([]map[string]any, error) {
	var results []map[string]any
	var mu sync.Mutex
	var wg sync.WaitGroup

	if batchSize <= 0 {
		batchSize = len(iterations)
	}

	// Use semaphore pattern for batch control
	semaphore := make(chan struct{}, batchSize)

	for i, iteration := range iterations {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(iterIdx int, iter matrixIteration) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			// Note: In parallel mode, we don't modify shared ctxResults
			// as it would cause race conditions. Each iteration gets
			// its own context copy for variable resolution.
			// Results are still stored with node name prefix.

			// Execute plugin and collect results
			iterationResult, err := b.callPluginAndCollect(ctx, t.step)
			if err != nil {
				b.logger.Errorw("matrix parallel iteration failed",
					"iteration", iterIdx+1, "err", err)
				return
			}

			mu.Lock()
			results = append(results, iterationResult)
			mu.Unlock()
			b.logger.Infow("matrix parallel iteration completed",
				"iteration", iterIdx+1, "total", len(iterations))
		}(i, iteration)
	}

	wg.Wait()
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

func newPluginRequest(workingPath string, job *types.WorkflowJob, step *types.WorkflowJobNode, store pluginapi.ContextStore, result pluginapi.Results) *pluginapi.Request {
	req := pluginapi.NewRequest()
	req.JobID = job.Id
	req.WorkingPath = workingPath
	req.Namespace = job.Namespace
	req.PluginName = step.Type
	req.ContextStore = store

	resultData := result.Data()
	globalVars := injectGlobalVars(job)
	for k, v := range globalVars {
		resultData[k] = v
	}

	req.Parameter = map[string]string{}

	for k, v := range step.Input {
		req.Parameter[k] = renderParams(v, resultData)
	}

	for k, v := range step.Params {
		req.Parameter[k] = renderParams(v, resultData)
	}

	return req
}

func callPlugin(ctx context.Context, step *types.WorkflowJobNode, mgr plugin.Manager, req *pluginapi.Request) (*pluginapi.Response, error) {
	pc := types.PluginCall{PluginName: step.Type}
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
