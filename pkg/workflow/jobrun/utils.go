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
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var (
	runnerExecTimeUsage = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "jobrun_runner_exec_time_usage_seconds",
			Help:    "The time usage of runner run.",
			Buckets: prometheus.ExponentialBuckets(0.1, 5, 5),
		},
	)
	runnerStartedCounter = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "jobrun_runner_started_gauge",
			Help: "This total of current started runner",
		},
	)
)

func init() {
	prometheus.MustRegister(
		runnerExecTimeUsage,
		runnerStartedCounter,
	)
}

type queue struct {
	q       chan string
	p       chan struct{}
	handler func(string)
	inQueue map[string]struct{}
	mux     sync.Mutex
}

func (q *queue) HandleNext(handler func(string)) {
	q.p <- struct{}{}
	next := <-q.q

	go func(jID string) {
		handler(jID)

		<-q.p
		q.mux.Lock()
		delete(q.inQueue, jID)
		q.mux.Unlock()
	}(next)
}

const (
	workQueueStatusAppended = "appended"
	workQueueStatusQueued   = "queued"
	workQueueStatusFilled   = "filled"
)

func (q *queue) Put(jobID string) (status string) {
	q.mux.Lock()
	defer q.mux.Unlock()
	if _, ok := q.inQueue[jobID]; ok {
		return workQueueStatusQueued
	}
	select {
	case q.q <- jobID:
		q.inQueue[jobID] = struct{}{}
		return workQueueStatusAppended
	default:
		return workQueueStatusFilled
	}
}

func targetHash(target types.WorkflowTarget) string {
	return fmt.Sprintf("t-p%d-e%d", target.ParentEntryID, target.EntryID)
}

func newQueue(size, parallel int) *queue {
	return &queue{
		q:       make(chan string, size),
		p:       make(chan struct{}, parallel),
		inQueue: make(map[string]struct{})}
}
