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

package metastore

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	metaOperationLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "meta_operation_latency_seconds",
			Help:    "The latency of meta store operation.",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		},
		[]string{"operation"},
	)
	metaOperationErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "meta_operation_errors",
			Help: "This count of meta store encountering errors",
		},
		[]string{"operation"},
	)

	disableMetrics bool
)

func init() {
	prometheus.MustRegister(
		metaOperationLatency,
		metaOperationErrorCounter,
	)
}

func DisableMetrics() {
	disableMetrics = true
}

func logOperationLatency(operation string, startAt time.Time) {
	metaOperationLatency.WithLabelValues(operation).Observe(time.Since(startAt).Seconds())
}

func logOperationError(operation string, err error) {
	if err != nil && err != context.Canceled {
		metaOperationErrorCounter.WithLabelValues(operation).Inc()
	}
}
