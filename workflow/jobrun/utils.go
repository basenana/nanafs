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
	"encoding/json"
	"strings"
	"time"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/ohler55/ojg/jp"
	"github.com/prometheus/client_golang/prometheus"
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

func logOperationLatency(execName, operation string, startAt time.Time) {
	execOperationTimeUsage.WithLabelValues(execName, operation).Observe(time.Since(startAt).Seconds())
}

func logOperationError(execName, operation string, err error) error {
	if err != nil && err != context.Canceled {
		execOperationErrorCounter.WithLabelValues(execName, operation).Inc()
	}
	return err
}

// getJSONPathValue extracts value from data using JSONPath syntax
// Example: "$.file_paths" or "file_paths" extracts file_paths from root
//
//	"$.nested.object.value" or "nested.object.value" extracts nested value
//
// Note: ojg library does not require the leading $ - it is implied
func getJSONPathValue(path string, data map[string]interface{}) (any, error) {
	// Remove leading $ if present since ojg library implies root
	expr := strings.TrimPrefix(path, "$")
	// Also remove leading . or [ if it follows $
	if len(expr) > 0 && (expr[0] == '.' || expr[0] == '[') {
		expr = expr[1:]
	}

	x, err := jp.ParseString(expr)
	if err != nil {
		return nil, err
	}
	// Pass data directly since Get() handles map[string]any
	result := x.Get(data)
	if len(result) == 0 {
		return "", nil
	}
	if len(result) > 1 {
		return result, nil
	}
	return result[0], nil
}

func renderParams(value any, data map[string]interface{}) any {
	strVal, ok := value.(string)
	if !ok {
		return value
	}

	if !strings.HasPrefix(strVal, "$") {
		return strVal
	}

	val, err := getJSONPathValue(strVal, data)
	if err != nil {
		return strVal
	}
	return val
}

// renderMatrixParam renders a single template reference and returns the value
// It handles both JSONPath references and direct template execution
func renderMatrixParam(value any, ctxData map[string]interface{}) any {
	strVal, ok := value.(string)
	if !ok {
		return value
	}

	if strings.HasPrefix(strVal, "$") {
		val, err := getJSONPathValue(strVal, ctxData)
		if err == nil {
			return val
		}
		return strVal
	}

	return strVal
}

// matrixIteration represents a single iteration's variable values
type matrixIteration struct {
	Variables map[string]any
}

// renderMatrixData parses matrix configuration and returns iteration data
// It extracts array values from context and generates Cartesian product if needed
func renderMatrixData(matrixData map[string]any, resultData map[string]any) ([]matrixIteration, error) {
	if len(matrixData) == 0 {
		return []matrixIteration{}, nil
	}

	var arrayVars = make(map[string][]any)

	// First pass: extract all array variables
	for varName, tplRef := range matrixData {
		// Render the template reference to get actual value
		rendered := renderMatrixParam(tplRef, resultData)

		// Check if rendered value is already an array
		if values, ok := rendered.([]any); ok {
			arrayVars[varName] = values
			continue
		}

		// Try to parse string as JSON array
		if strVal, ok := rendered.(string); ok {
			var values []any
			if err := json.Unmarshal([]byte(strVal), &values); err == nil {
				arrayVars[varName] = values
			}
		}
	}

	// If no array variables found, return empty
	if len(arrayVars) == 0 {
		return []matrixIteration{}, nil
	}

	// Generate Cartesian product
	var (
		iterations []matrixIteration
		maxLen     int
	)
	for _, values := range arrayVars {
		if len(values) > maxLen {
			maxLen = len(values)
		}
	}

	for i := 0; i < maxLen; i++ {
		ite := matrixIteration{Variables: make(map[string]any)}
		for key, values := range arrayVars {
			if len(values) <= i {
				continue
			}
			ite.Variables[key] = values[i]
		}
		if len(ite.Variables) == 0 {
			break
		}
		iterations = append(iterations, ite)
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
