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

package core

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	entryOperationLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dentry_operation_latency_seconds",
			Help:    "The latency of entry operation.",
			Buckets: prometheus.ExponentialBuckets(0.0001, 5, 5),
		},
		[]string{"operation"},
	)
	entryOperationErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dentry_operation_errors",
			Help: "This count of entry operation encountering errors",
		},
		[]string{"operation"},
	)
	fileOperationLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dentry_file_operation_latency_seconds",
			Help:    "The latency of file operation.",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2.5, 15),
		},
		[]string{"operation"},
	)
	fileOperationErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dentry_file_operation_errors",
			Help: "This count of file operation encountering errors",
		},
		[]string{"operation"},
	)
	groupOperationLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dentry_group_operation_latency_seconds",
			Help:    "The latency of group operation.",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"operation"},
	)
	groupOperationErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dentry_group_operation_errors",
			Help: "This count of group operation encountering errors",
		},
		[]string{"operation"},
	)
)

func init() {
	prometheus.MustRegister(
		entryOperationLatency,
		entryOperationErrorCounter,
		fileOperationLatency,
		fileOperationErrorCounter,
		groupOperationLatency,
		groupOperationErrorCounter,
	)
}

func logOperationLatency(h *prometheus.HistogramVec, operation string, startAt time.Time) {
	h.WithLabelValues(operation).Observe(time.Since(startAt).Seconds())
}

func logOperationError(counter *prometheus.CounterVec, operation string, err error) error {
	if err != nil && err != context.Canceled {
		counter.WithLabelValues(operation).Inc()
	}
	return err
}
