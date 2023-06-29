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

package storage

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	storageOperationLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storage_operation_latency_seconds",
			Help:    "The latency of storage operation.",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 15),
		},
		[]string{"storage_id", "operation"},
	)
	storageOperationErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storage_operation_errors",
			Help: "This count of storage encountering errors",
		},
		[]string{"storage_id", "operation"},
	)
	localCacheOperationLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "local_cache_operation_latency_seconds",
			Help:    "The latency of local cache operation.",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 15),
		},
		[]string{"operation"},
	)
	localCacheOperationErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "local_cache_operation_errors",
			Help: "This count of local cache encountering errors",
		},
		[]string{"operation"},
	)
	localCachedNodeGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "local_cache_nodes",
			Help: "This count of local cached nodes",
		},
	)
	localCachedUsageGauge = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "local_cache_usage_bytes",
			Help: "This usage of local cached nodes",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&localCacheSizeUsage))
		},
	)
	localCachedLimitGauge = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "local_cache_limit_bytes",
			Help: "This limit of local cached nodes",
		},
		func() float64 {
			return float64(localCacheSizeLimit)
		},
	)
	localCacheFetchingGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "local_cache_fetching_node",
			Help: "This count of cache fetching",
		},
	)
)

func init() {
	prometheus.MustRegister(
		storageOperationLatency,
		storageOperationErrorCounter,
		localCacheOperationLatency,
		localCacheOperationErrorCounter,
		localCachedNodeGauge,
		localCachedUsageGauge,
		localCachedLimitGauge,
		localCacheFetchingGauge,
	)
}

type instrumentalStorage struct {
	s Storage
}

func (i instrumentalStorage) ID() string {
	return i.s.ID()
}

func (i instrumentalStorage) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	const getOperation = "get"
	defer logStorageOperationLatency(i.ID(), getOperation, time.Now())
	r, err := i.s.Get(ctx, key, idx)
	return r, logErr(storageOperationErrorCounter, err, i.ID(), getOperation)
}

func (i instrumentalStorage) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	const putOperation = "put"
	defer logStorageOperationLatency(i.ID(), putOperation, time.Now())
	err := i.s.Put(ctx, key, idx, dataReader)
	return logErr(storageOperationErrorCounter, err, i.ID(), putOperation)
}

func (i instrumentalStorage) Delete(ctx context.Context, key int64) error {
	const deleteOperation = "delete"
	defer logStorageOperationLatency(i.ID(), deleteOperation, time.Now())
	err := i.s.Delete(ctx, key)
	return logErr(storageOperationErrorCounter, err, i.ID(), deleteOperation)
}

func (i instrumentalStorage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	const headOperation = "head"
	defer logStorageOperationLatency(i.ID(), headOperation, time.Now())
	info, err := i.s.Head(ctx, key, idx)
	return info, logErr(storageOperationErrorCounter, err, i.ID(), headOperation)
}

func logStorageOperationLatency(id, operation string, startAt time.Time) {
	storageOperationLatency.WithLabelValues(id, operation).Observe(time.Since(startAt).Seconds())
}

func logLocalCacheOperationLatency(operation string, startAt time.Time) {
	localCacheOperationLatency.WithLabelValues(operation).Observe(time.Since(startAt).Seconds())
}

func logErr(counter *prometheus.CounterVec, err error, labels ...string) error {
	if err != nil {
		counter.WithLabelValues(labels...).Inc()
	}
	return err
}
