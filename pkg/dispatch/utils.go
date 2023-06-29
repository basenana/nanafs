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

package dispatch

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

var (
	taskExecutionLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dispatch_task_execution_latency_seconds",
			Help:    "The latency of dispatch task execution.",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		},
		[]string{"id"},
	)
	taskExecutionErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dispatch_task_execution_errors",
			Help: "This count of dispatch task encountering errors",
		},
	)
	taskFinishStatusCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dispatch_task_finished",
			Help: "This count of storage encountering errors",
		},
		[]string{"id", "status"},
	)
	taskCurrentRunningGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dispatch_task_current_running",
			Help: "This count of waiting or running tasks",
		},
		[]string{"id", "status"},
	)
)

func init() {
	prometheus.MustRegister(
		taskExecutionLatency,
		taskExecutionErrorCounter,
		taskFinishStatusCounter,
		taskCurrentRunningGauge,
	)
}

func getWaitingTask(ctx context.Context, recorder metastore.ScheduledTaskRecorder, taskID string, evt *types.Event) (*types.ScheduledTask, error) {
	tasks, err := recorder.ListTask(ctx, taskID,
		types.ScheduledTaskFilter{RefType: evt.RefType, RefID: evt.RefID, Status: []string{types.ScheduledTaskInitial, types.ScheduledTaskWait}})
	if err != nil {
		return nil, fmt.Errorf("list waiting task error: %s", err)
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	return tasks[0], nil
}

func logTaskExecutionLatency(id string, startAt time.Time) {
	taskExecutionLatency.WithLabelValues(id).Observe(time.Since(startAt).Seconds())
}
