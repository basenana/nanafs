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

package bio

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"sync/atomic"
	"time"
)

var (
	chunkReaderLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "io_chunk_reader_latency_seconds",
			Help:    "The latency of chunk reader.",
			Buckets: prometheus.ExponentialBuckets(0.00001, 5, 10),
		},
		[]string{"step"},
	)
	chunkWriterLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "io_chunk_writer_latency_seconds",
			Help:    "The latency of chunk writer.",
			Buckets: prometheus.ExponentialBuckets(0.0000001, 5, 10),
		},
		[]string{"step"},
	)
	chunkCommitSegmentLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "io_chunk_commit_segment_latency_seconds",
			Help:    "The latency of commit chunk segment.",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 15),
		},
		[]string{"step"},
	)
	chunkReadErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "io_chunk_read_errors",
			Help: "This count of chunk read encountering errors,",
		},
		[]string{"step"},
	)
	chunkWriteErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "io_chunk_write_errors",
			Help: "This count of chunk write encountering errors.",
		},
		[]string{"step"},
	)
	chunkDiscardSegmentCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "io_chunk_discard_segments",
			Help: "This count of chunk upload segment failed.",
		},
	)
	chunkReadingGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "io_chunk_reading",
			Help: "This count of reading goroutines.",
		},
	)
	chunkMergePageCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "io_chunk_reader_merge",
			Help: "This count of reader merge pages.",
		},
	)
	chunkDirtyPageGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "io_chunk_dirty_pages",
			Help: "This count of dirty page.",
		},
	)
	chunkUncommittedGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "io_chunk_uncommitted_segments",
			Help: "This count of uncommitted segments.",
		},
	)
	pageCacheCountGauge = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "io_current_page_caches",
			Help: "This current count of page caches",
		},
		func() float64 {
			return float64(atomic.LoadInt32(&crtPageCacheTotal))
		},
	)
	pageCacheLimitGauge = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "io_page_cache_limit_bytes",
			Help: "This limit count of page caches,",
		},
		func() float64 {
			return float64(maxPageCacheTotal)
		},
	)
)

func init() {
	prometheus.MustRegister(
		chunkReaderLatency,
		chunkWriterLatency,
		chunkCommitSegmentLatency,
		chunkReadErrorCounter,
		chunkWriteErrorCounter,
		chunkDiscardSegmentCounter,
		chunkReadingGauge,
		chunkDirtyPageGauge,
		chunkUncommittedGauge,
		pageCacheCountGauge,
		pageCacheLimitGauge,
	)
}

func logLatency(h *prometheus.HistogramVec, step string, startAt time.Time) {
	h.WithLabelValues(step).Observe(time.Since(startAt).Seconds())
}

func logErr(counter *prometheus.CounterVec, err error, step string) error {
	if err != nil && err != context.Canceled {
		counter.WithLabelValues(step).Inc()
	}
	return err
}
