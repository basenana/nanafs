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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
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

type targetEntry struct {
	entry  *types.Entry
	parent *types.Entry
}

func initParentDirCacheData(ctx context.Context, namespace string, fsCore core.Core, parentEntryID int64) (*pluginapi.CachedData, error) {
	cachedDataCh, err := fsCore.FindEntry(ctx, namespace, parentEntryID, pluginapi.CachedDataFile)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, fmt.Errorf("find cached data entry %s failed: %s", pluginapi.CachedDataFile, err)
	}

	if cachedDataCh != nil {
		cachedDataFile, err := fsCore.Open(ctx, namespace, cachedDataCh.ChildID, types.OpenAttr{Read: true})
		if err != nil {
			return nil, fmt.Errorf("open cached data entry %d failed: %s", cachedDataCh.ChildID, err)
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

func writeParentDirCacheData(ctx context.Context, namespace string, fsCore core.Core, parentEntryID int64, data *pluginapi.CachedData) error {
	if !data.NeedReCache() {
		return nil
	}

	cachedDataCh, err := fsCore.FindEntry(ctx, namespace, parentEntryID, pluginapi.CachedDataFile)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return fmt.Errorf("find cached data entry %s failed: %s", pluginapi.CachedDataFile, err)
	}

	var cachedDataEnID int64
	if cachedDataCh == nil {
		cachedDataEn, err := fsCore.CreateEntry(ctx, namespace, parentEntryID, types.EntryAttr{Name: pluginapi.CachedDataFile, Kind: types.RawKind})
		if err != nil {
			return fmt.Errorf("create new cached data entry failed: %s", err)
		}
		cachedDataEnID = cachedDataEn.ID
	} else {
		cachedDataEnID = cachedDataCh.ChildID
	}

	newReader, err := data.Reader()
	if err != nil {
		return fmt.Errorf("open cached data entry reader failed: %s", err)
	}

	f, err := fsCore.Open(ctx, namespace, cachedDataEnID, types.OpenAttr{Write: true, Trunc: true})
	if err != nil {
		return fmt.Errorf("open cached data entry %d failed: %s", cachedDataEnID, err)
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

func logOperationLatency(execName, operation string, startAt time.Time) {
	execOperationTimeUsage.WithLabelValues(execName, operation).Observe(time.Since(startAt).Seconds())
}

func logOperationError(execName, operation string, err error) error {
	if err != nil && err != context.Canceled {
		execOperationErrorCounter.WithLabelValues(execName, operation).Inc()
	}
	return err
}

func renderParams(tplContent string, data map[string]interface{}) string {
	if !strings.Contains(tplContent, "{{") {
		return tplContent
	}
	tpl, err := template.New("params").Delims("{{", "}}").Parse(tplContent)
	if err != nil {
		return tplContent
	}

	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, data)
	if err != nil {
		return tplContent
	}
	return buf.String()
}

// renderMatrixParam renders a single template reference and returns the value
// It handles both map key access and direct template execution
func renderMatrixParam(tplRef string, ctxData map[string]interface{}) string {
	// For simple variable references like "{{ file_paths }}", extract the variable name
	if strings.HasPrefix(tplRef, "{{") && strings.HasSuffix(tplRef, "}}") {
		varName := strings.TrimSpace(strings.TrimPrefix(tplRef, "{{"))
		varName = strings.TrimSpace(strings.TrimSuffix(varName, "}}"))
		if val, ok := ctxData[varName]; ok {
			// If value is a slice (array), marshal to JSON for parsing
			if sliceVal, ok := val.([]any); ok {
				jsonBytes, err := json.Marshal(sliceVal)
				if err == nil {
					return string(jsonBytes)
				}
			}
			// Return the value as string representation
			return fmt.Sprintf("%v", val)
		}
		return tplRef
	}

	// For complex templates, use the template engine
	return renderParams(tplRef, ctxData)
}

// matrixIteration represents a single iteration's variable values
type matrixIteration struct {
	Variables map[string]any
}

// renderMatrixData parses matrix configuration and returns iteration data
// It extracts array values from context and generates Cartesian product if needed
func renderMatrixData(matrixData map[string]string, ctxResults pluginapi.Results) ([]matrixIteration, error) {
	if len(matrixData) == 0 {
		return nil, fmt.Errorf("matrix data is empty")
	}

	ctxData := ctxResults.Data()
	var arrayVars []struct {
		name   string
		values []any
	}

	// First pass: extract all array variables
	for varName, tplRef := range matrixData {
		// Render the template reference to get actual value
		rendered := renderMatrixParam(tplRef, ctxData)
		if rendered == tplRef {
			// Template not resolved, skip or treat as scalar
			continue
		}

		// Try to parse as JSON array
		var values []any
		if err := json.Unmarshal([]byte(rendered), &values); err != nil {
			// Not an array, skip for array extraction
			continue
		}

		arrayVars = append(arrayVars, struct {
			name   string
			values []any
		}{name: varName, values: values})
	}

	// If no array variables found, return empty
	if len(arrayVars) == 0 {
		return nil, fmt.Errorf("no array variables found in matrix data")
	}

	// Generate Cartesian product
	var iterations []matrixIteration

	if len(arrayVars) == 1 {
		// Single array - simple iteration
		for _, val := range arrayVars[0].values {
			iterations = append(iterations, matrixIteration{
				Variables: map[string]any{
					arrayVars[0].name: val,
				},
			})
		}
	} else {
		// Multiple arrays - Cartesian product
		// Recursive helper for Cartesian product
		var cartesian func(idx int, current map[string]any)
		cartesian = func(idx int, current map[string]any) {
			if idx == len(arrayVars) {
				iterations = append(iterations, matrixIteration{
					Variables: copyMap(current),
				})
				return
			}
			for _, val := range arrayVars[idx].values {
				current[arrayVars[idx].name] = val
				cartesian(idx+1, current)
			}
		}
		cartesian(0, make(map[string]any))
	}

	return iterations, nil
}

func copyMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func injectGlobalVars(job *types.WorkflowJob) map[string]interface{} {
	return map[string]interface{}{
		"job_id":           job.Id,
		"workflow_id":      job.Workflow,
		"timestamp":        time.Now().Unix(),
		"timestampRFC3339": time.Now().Format(time.RFC3339),
	}
}
