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
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/basenana/go-flow"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
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
			job:      job,
			entryMgr: ctrl.entryMgr,
			docMgr:   ctrl.docMgr,
			config:   ctrl.config,
			logger:   logger.NewLogger("pipeExecutor").With(zap.String("job", job.Id)),
		}
	default:
		return &fileExecutor{
			job:      job,
			entryMgr: ctrl.entryMgr,
			docMgr:   ctrl.docMgr,
			config:   ctrl.config,
			logger:   logger.NewLogger("fileExecutor").With(zap.String("job", job.Id)),
		}
	}
}

type pipeExecutor struct {
	job      *types.WorkflowJob
	entryMgr dentry.Manager
	docMgr   document.Manager
	config   Config
	logger   *zap.SugaredLogger

	ctxResults pluginapi.Results
	targets    []*types.Metadata
}

var _ flow.Executor = &pipeExecutor{}

func (p *pipeExecutor) Setup(ctx context.Context) error {
	var (
		en  *types.Metadata
		err error
	)

	p.ctxResults = pluginapi.NewMemBasedResults()
	for _, eid := range p.job.Target.Entries {
		en, err = p.entryMgr.GetEntry(ctx, eid)
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
	req := newPluginRequest(p.job, t.step, p.ctxResults)
	for _, en := range p.targets {
		req.Entries = append(req.Entries, pluginapi.Entry{
			ID:         en.ID,
			Name:       en.Name,
			Kind:       en.Kind,
			Size:       en.Size,
			IsGroup:    en.IsGroup,
			Parameters: make(map[string]string),
		})
	}
	err = callPlugin(ctx, p.job, *t.step.Plugin, p.entryMgr, req, p.ctxResults, p.logger)
	if err != nil {
		return logOperationError(DataPipeExecName, "call_plugin", err)
	}
	return
}

func (p *pipeExecutor) Teardown(ctx context.Context) error {
	return nil
}

type fileExecutor struct {
	job      *types.WorkflowJob
	entryMgr dentry.Manager
	docMgr   document.Manager
	config   Config

	workdir    string
	entryPath  string
	entryURI   string
	cachedData *pluginapi.CachedData

	ctxResults pluginapi.Results
	logger     *zap.SugaredLogger
}

var _ flow.Executor = &fileExecutor{}

func (b *fileExecutor) Setup(ctx context.Context) (err error) {
	startAt := time.Now()
	defer logOperationLatency(FileExecName, "setup", startAt)

	// init workdir and copy entry file
	b.workdir, err = initWorkdir(ctx, b.config.JobWorkdir, b.job)
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
		epath, err := entryWorkdirInit(ctx, enID, b.entryMgr, b.workdir)
		if err != nil {
			b.logger.Errorw("copy target file to workdir failed", "err", err, "entry", enID)
			return logOperationError(FileExecName, "setup", err)
		}
		b.logger.Infow("copy entry to workdir", "entry", enID, "path", epath)
	}

	if b.job.Target.ParentEntryID != 0 {
		// base on parent entry
		b.cachedData, err = initParentDirCacheData(ctx, b.entryMgr, b.job.Target.ParentEntryID)
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

	req := newPluginRequest(b.job, t.step, b.ctxResults)
	req.WorkPath = b.workdir
	req.CacheData = b.cachedData
	req.ContextResults = b.ctxResults

	err = callPlugin(ctx, b.job, *t.step.Plugin, b.entryMgr, req, b.ctxResults, b.logger)
	return
}

func (b *fileExecutor) Collect(ctx context.Context) error {
	startAt := time.Now()
	defer logOperationLatency(FileExecName, "collect", startAt)
	if b.ctxResults.IsSet(pluginapi.ResCollectManifests) {
		var manifests []pluginapi.CollectManifest
		if err := b.ctxResults.Load(pluginapi.ResCollectManifests, &manifests); err != nil {
			msg := fmt.Sprintf("collect manifest objects failed: %s", err)
			b.logger.Error(msg)
			return logOperationError(FileExecName, "collect", fmt.Errorf(msg))
		}
		// collect files
		err := b.collectFiles(ctx, manifests)
		if err != nil {
			return err
		}
	}

	if b.cachedData != nil && b.cachedData.NeedReCache() {
		b.logger.Infow("collect cache data")
		if err := writeParentDirCacheData(ctx, b.entryMgr, b.job.Target.ParentEntryID, b.cachedData); err != nil {
			b.logger.Errorw("write parent cached data back failed", "err", err)
			return err
		}
	}

	if b.ctxResults.IsSet(pluginapi.ResEntryDocumentsKey) {
		var docs []types.FDocument
		if err := b.ctxResults.Load(pluginapi.ResEntryDocumentsKey, &docs); err != nil {
			msg := fmt.Sprintf("collect document objects failed: %s", err)
			b.logger.Error(msg)
			return logOperationError(FileExecName, "collect", fmt.Errorf(msg))
		}

		buf := bytes.Buffer{}
		for _, doc := range docs {
			buf.WriteString(doc.Content)
			buf.WriteString("\n")
		}

		// collect documents
		var entryID = b.job.Target.EntryID
		err := b.collectDocuments(ctx, entryID, buf)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *fileExecutor) collectFiles(ctx context.Context, manifests []pluginapi.CollectManifest) error {
	b.logger.Infow("collect files", "manifests", len(manifests))
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
		return logOperationError(FileExecName, "collect", err)
	}
	return nil
}

func (b *fileExecutor) collectDocuments(ctx context.Context, entryId int64, content bytes.Buffer) error {
	b.logger.Infow("collect documents", "entryId", entryId)
	if err := collectFile2Document(ctx, b.docMgr, b.entryMgr, entryId, content); err != nil {
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

func callPlugin(ctx context.Context, job *types.WorkflowJob, ps types.PlugScope,
	entryMgr dentry.Manager, req *pluginapi.Request, ctxResults pluginapi.Results, logger *zap.SugaredLogger) (err error) {
	if job.Target.ParentEntryID != 0 {
		ed, err := entryMgr.GetEntryExtendData(ctx, job.Target.ParentEntryID)
		if err != nil {
			err = fmt.Errorf("get parent entry extend data error: %s", err)
			return err
		}
		if ed.PlugScope != nil {
			ps = mergeParentEntryPlugScope(ps, *ed.PlugScope)
		}

		properties, err := entryMgr.ListEntryProperty(ctx, job.Target.ParentEntryID)
		if err != nil {
			err = fmt.Errorf("get parent entry properties error: %w", err)
			return err
		}
		for k, v := range properties.Fields {
			val := v.Value
			if v.Encoded {
				val, err = utils.DecodeBase64String(val)
				if err != nil {
					logger.Warnw("decode extend property value failed", "key", k)
					continue
				}
			}
			req.ParentProperties[k] = val
		}
	}

	if ps.PluginType == types.TypeSource {
		info, err := plugin.SourceInfo(ctx, ps)
		if err != nil {
			err = fmt.Errorf("get source info error: %s", err)
			return err
		}
		logger.Infow("running source plugin", "plugin", ps.PluginName, "source", info)
	}

	req.Action = ps.Action
	resp, err := plugin.Call(ctx, ps, req)
	if err != nil {
		err = fmt.Errorf("plugin action error: %s", err)
		return err
	}
	if !resp.IsSucceed {
		err = fmt.Errorf("plugin action failed: %s", resp.Message)
		return err
	}
	if len(resp.Results) > 0 {
		if err = ctxResults.SetAll(resp.Results); err != nil {
			logger.Errorw("set context result error", "err", err)
			return err
		}
	}
	return err
}
