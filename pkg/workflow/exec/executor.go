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
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/basenana/nanafs/utils"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	LocalExecName = "local"
)

func RegisterOperators(entryMgr dentry.Manager, docMgr document.Manager, cfg LocalConfig) error {
	jobrun.RegisterExecutorBuilder(LocalExecName, func(job *types.WorkflowJob) jobrun.Executor {
		return &localExecutor{
			job:      job,
			entryMgr: entryMgr,
			docMgr:   docMgr,
			config:   cfg,
			results:  map[string]any{},
			logger:   logger.NewLogger("localExecutor").With(zap.String("job", job.Id)),
		}
	})
	return nil
}

type localExecutor struct {
	job        *types.WorkflowJob
	workdir    string
	entryPath  string
	entryURI   string
	entryMgr   dentry.Manager
	docMgr     document.Manager
	cachedData *pluginapi.CachedData
	config     LocalConfig
	results    map[string]any
	resultMux  sync.Mutex
	logger     *zap.SugaredLogger
}

var _ jobrun.Executor = &localExecutor{}

func (b *localExecutor) Setup(ctx context.Context) (err error) {
	if !b.config.Workflow.Enable {
		return fmt.Errorf("workflow disabled")
	}

	startAt := time.Now()
	defer logOperationLatency(LocalExecName, "setup", startAt)

	// init workdir and copy entry file
	b.workdir, err = initWorkdir(ctx, b.config.Workflow.JobWorkdir, b.job)
	if err != nil {
		b.logger.Errorw("init job workdir failed", "err", err)
		return logOperationError(LocalExecName, "setup", err)
	}

	if b.job.Target.EntryID != 0 {
		b.entryPath, err = entryWorkdirInit(ctx, b.job.Target.EntryID, b.entryMgr, b.workdir)
		if err != nil {
			b.logger.Errorw("copy target file to workdir failed", "err", err)
			return logOperationError(LocalExecName, "setup", err)
		}

		b.entryURI, err = entryURIByEntryID(ctx, b.job.Target.EntryID, b.entryMgr)
		if err != nil {
			b.logger.Errorw("query entry dir failed", "err", err)
			return logOperationError(LocalExecName, "setup", err)
		}
	} else if b.job.Target.ParentEntryID != 0 {
		// base on parent entry
		b.cachedData, err = initParentDirCacheData(ctx, b.entryMgr, b.job.Target.ParentEntryID)
		if err != nil {
			b.logger.Errorw("build parent cache data failed", "parent", b.job.Target.ParentEntryID, "err", err)
			return logOperationError(LocalExecName, "setup", err)
		}
	}
	b.logger.Infow("job setup", "workdir", b.workdir, "entryPath", b.entryPath)

	return
}

func (b *localExecutor) DoOperation(ctx context.Context, step types.WorkflowJobStep) (err error) {
	startAt := time.Now()
	defer logOperationLatency(LocalExecName, "do_operation", startAt)

	defer func() {
		if panicErr := utils.Recover(); panicErr != nil {
			b.logger.Errorw("executor panic", "err", panicErr)
			err = panicErr
		}
	}()

	req := pluginapi.NewRequest()
	req.WorkPath = b.workdir
	req.EntryId = b.job.Target.EntryID
	req.ParentEntryId = b.job.Target.ParentEntryID
	req.CacheData = b.cachedData
	req.EntryPath = b.entryPath
	req.EntryURI = b.entryURI

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
	req.ParentProperties = map[string]string{}

	ps := *step.Plugin
	if b.job.Target.ParentEntryID != 0 {
		ed, err := b.entryMgr.GetEntryExtendData(ctx, b.job.Target.ParentEntryID)
		if err != nil {
			err = fmt.Errorf("get parent entry extend data error: %s", err)
			return logOperationError(LocalExecName, "do_operation", err)
		}
		if ed.PlugScope != nil {
			ps = mergeParentEntryPlugScope(ps, *ed.PlugScope)
		}
		if ed.Properties.Fields != nil {
			for k, v := range ed.Properties.Fields {
				val := v.Value
				if v.Encoded {
					val, err = utils.DecodeBase64String(val)
					if err != nil {
						b.logger.Warnw("decode extend property value failed", "key", k)
						continue
					}
				}
				req.ParentProperties[k] = val
			}
		}
	}

	if step.Plugin.PluginType == types.TypeSource {
		info, err := plugin.SourceInfo(ctx, ps)
		if err != nil {
			err = fmt.Errorf("get source info error: %s", err)
			return logOperationError(LocalExecName, "do_operation", err)
		}
		b.logger.Infow("running source plugin", "plugin", step.Plugin.PluginName, "source", info)
	}

	req.Action = ps.Action
	resp, err := plugin.Call(ctx, ps, req)
	if err != nil {
		err = fmt.Errorf("plugin action error: %s", err)
		return logOperationError(LocalExecName, "do_operation", err)
	}
	if !resp.IsSucceed {
		err = fmt.Errorf("plugin action failed: %s", resp.Message)
		return logOperationError(LocalExecName, "do_operation", err)
	}
	if len(resp.Results) > 0 {
		b.resultMux.Lock()
		for k, v := range resp.Results {
			b.results[k] = v
		}
		b.resultMux.Unlock()
	}
	return err
}

func (b *localExecutor) Collect(ctx context.Context) error {
	b.resultMux.Lock()
	rawManifests, needManifestCollect := b.results[pluginapi.ResCollectManifests]
	rawDocs, needDocCollect := b.results[pluginapi.ResEntryDocumentsKey]
	b.resultMux.Unlock()
	if !needDocCollect && !needManifestCollect {
		return nil
	}

	startAt := time.Now()
	defer logOperationLatency(LocalExecName, "collect", startAt)

	if needManifestCollect {
		manifests, ok := rawManifests.([]pluginapi.CollectManifest)
		if !ok {
			msg := fmt.Sprintf("load collect manifest objects failed: unknown type %v", rawManifests)
			b.logger.Error(msg)
			return logOperationError(LocalExecName, "collect", fmt.Errorf(msg))
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

	if needDocCollect {
		docs, ok := rawDocs.([]types.FDocument)
		if !ok || len(docs) == 0 {
			return fmt.Errorf("content is empty")
		}

		buf := bytes.Buffer{}
		for _, doc := range docs {
			buf.WriteString(doc.Content)
			buf.WriteString("\n")
		}

		// collect documents
		var (
			entryID    int64
			summary    string
			keyWords   []string
			totalUsage = make(map[string]any)
		)
		if _, ok := b.results[pluginapi.ResEntryIdKey]; !ok {
			return fmt.Errorf("content is empty")
		}
		entryID = b.results[pluginapi.ResEntryIdKey].(int64)
		if _, ok := b.results[pluginapi.ResEntryURIKey]; !ok {
			return fmt.Errorf("content is empty")
		}

		if _, ok := b.results[pluginapi.ResEntryDocSummaryKey]; ok {
			summaryVal := b.results[pluginapi.ResEntryDocSummaryKey].(map[string]any)
			summary = summaryVal["summary"].(string)
			totalUsage["summary"] = summaryVal["usage"]
		}
		if _, ok := b.results[pluginapi.ResEntryDocKeyWordsKey]; ok {
			keyWordsVal := b.results[pluginapi.ResEntryDocKeyWordsKey].(map[string]any)
			keyWords = keyWordsVal["keywords"].([]string)
			totalUsage["keywords"] = keyWordsVal["usage"]
		}
		err := b.collectDocuments(ctx, entryID, buf, summary, keyWords, totalUsage)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *localExecutor) collectFiles(ctx context.Context, manifests []pluginapi.CollectManifest) error {
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
		return logOperationError(LocalExecName, "collect", err)
	}
	return nil
}

func (b *localExecutor) collectDocuments(ctx context.Context, entryId int64, content bytes.Buffer, summary string, keyWords []string, usage map[string]any) error {
	b.logger.Infow("collect documents", "entryId", entryId)
	if err := collectFile2Document(ctx, b.docMgr, b.entryMgr, entryId, content, summary, keyWords, usage); err != nil {
		return logOperationError(LocalExecName, "collect", err)
	}
	return nil
}

func (b *localExecutor) Teardown(ctx context.Context) {
	startAt := time.Now()
	defer logOperationLatency(LocalExecName, "teardown", startAt)
	err := cleanupWorkdir(ctx, b.workdir)
	if err != nil {
		b.logger.Errorw("teardown failed: cleanup workdir error", "err", err)
		_ = logOperationError(LocalExecName, "teardown", err)
		return
	}
}

type LocalConfig struct {
	Workflow config.Workflow
}
