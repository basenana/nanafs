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
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"time"
)

var (
	execOperationTimeUsage = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "exec_operation_time_usage_seconds",
			Help:    "The time usage of do operation.",
			Buckets: prometheus.ExponentialBuckets(0.05, 5, 5),
		},
		[]string{"exec_name", "operation"},
	)
	execOperationErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "exec_operation_errors",
			Help: "This count of exec encountering errors",
		},
		[]string{"exec_name", "operation"},
	)
)

func init() {
	prometheus.MustRegister(
		execOperationTimeUsage,
		execOperationErrorCounter,
	)
}

func initParentDirCacheData(ctx context.Context, entryMgr dentry.Manager, parentEntryID int64) (*pluginapi.CachedData, error) {
	parent, err := entryMgr.OpenGroup(ctx, parentEntryID)
	if err != nil {
		return nil, fmt.Errorf("open parent %d to init failed: %s", parentEntryID, err)
	}
	cachedDataEn, err := parent.FindEntry(ctx, pluginapi.CachedDataFile)
	if err != nil && err != types.ErrNotFound {
		return nil, fmt.Errorf("find cached data entry %s failed: %s", pluginapi.CachedDataFile, err)
	}

	if cachedDataEn != nil {
		cachedDataFile, err := entryMgr.Open(ctx, cachedDataEn.ID, types.OpenAttr{Read: true})
		if err != nil {
			return nil, fmt.Errorf("open cached data entry %d failed: %s", cachedDataEn.ID, err)
		}

		cachedData, err := pluginapi.OpenCacheData(utils.NewReaderWithContextReaderAt(ctx, cachedDataFile))
		if err != nil {
			_ = cachedDataFile.Close(ctx)
			return nil, fmt.Errorf("load cached entry failed: %s", err)
		}
		return cachedData, cachedDataFile.Close(ctx)
	}
	return pluginapi.InitCacheData(), nil
}

func writeParentDirCacheData(ctx context.Context, entryMgr dentry.Manager, parentEntryID int64, data *pluginapi.CachedData) error {
	if !data.NeedReCache() {
		return nil
	}

	parent, err := entryMgr.OpenGroup(ctx, parentEntryID)
	if err != nil {
		return fmt.Errorf("open parent %d to write cache failed: %s", parentEntryID, err)
	}
	cachedDataEn, err := parent.FindEntry(ctx, pluginapi.CachedDataFile)
	if err != nil && err != types.ErrNotFound {
		return fmt.Errorf("find cached data entry %s failed: %s", pluginapi.CachedDataFile, err)
	}

	if cachedDataEn == nil {
		cachedDataEn, err = parent.CreateEntry(ctx, types.EntryAttr{Name: pluginapi.CachedDataFile, Kind: types.RawKind})
		if err != nil {
			return fmt.Errorf("create new cached data entry failed: %s", err)
		}
	}

	newReader, err := data.Reader()
	if err != nil {
		return fmt.Errorf("open cached data entry reader failed: %s", err)
	}

	f, err := entryMgr.Open(ctx, cachedDataEn.ID, types.OpenAttr{Write: true, Trunc: true})
	if err != nil {
		return fmt.Errorf("open cached data entry %d failed: %s", cachedDataEn.ID, err)
	}

	_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, f), newReader)
	if err != nil {
		_ = f.Close(ctx)
		return fmt.Errorf("copy cached new content to entry cahed data file failed: %s", err)
	}

	err = f.Close(ctx)
	if err != nil {
		return fmt.Errorf("close cahed data file failed: %s", err)
	}
	return nil
}

func mergeParentEntryPlugScope(step, entryDef types.PlugScope) types.PlugScope {
	if step.PluginName != entryDef.PluginName {
		return step
	}
	ps := types.PlugScope{
		PluginName: step.PluginName,
		Version:    step.Version,
		PluginType: step.PluginType,
		Action:     entryDef.Action,
		Parameters: map[string]string{},
	}

	if entryDef.Parameters != nil {
		for k, v := range entryDef.Parameters {
			ps.Parameters[k] = v
		}
	}

	// do overwrite
	if step.Action != "" {
		ps.Action = step.Action
	}
	if step.Parameters != nil {
		for k, v := range step.Parameters {
			ps.Parameters[k] = v
		}
	}
	return ps
}

func logOperationLatency(execName, operation string, startAt time.Time) {
	execOperationTimeUsage.WithLabelValues(execName, operation).Observe(time.Since(startAt).Seconds())
}

func logOperationError(execName, operation string, err error) error {
	if err != nil && err != context.Canceled {
		execOperationErrorCounter.WithLabelValues(execName, operation).Inc()
	}
	return err
}

func newPluginRequest(job *types.WorkflowJob, step *types.WorkflowJobStep, result pluginapi.Results) *pluginapi.Request {
	req := pluginapi.NewRequest()
	req.ParentEntryId = job.Target.ParentEntryID
	req.ContextResults = result
	req.Namespace = job.Namespace

	req.Parameter = map[string]any{}
	for k, v := range step.Plugin.Parameters {
		req.Parameter[k] = v
	}
	req.Parameter[pluginapi.ResPluginName] = step.Plugin.PluginName
	req.Parameter[pluginapi.ResPluginVersion] = step.Plugin.Version
	req.Parameter[pluginapi.ResPluginType] = step.Plugin.PluginType
	req.Parameter[pluginapi.ResPluginAction] = step.Plugin.Action
	req.ParentProperties = map[string]string{}
	return req
}
