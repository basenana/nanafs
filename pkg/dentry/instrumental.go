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

package dentry

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"time"
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

type instrumentalEntry struct {
	en Entry
}

func (i instrumentalEntry) Metadata() *types.Metadata {
	return i.en.Metadata()
}

func (i instrumentalEntry) GetExtendData(ctx context.Context) (types.ExtendData, error) {
	const operation = "get_extend_data"
	defer logOperationLatency(entryOperationLatency, operation, time.Now())
	ed, err := i.en.GetExtendData(ctx)
	return ed, logOperationError(entryOperationErrorCounter, operation, err)
}

func (i instrumentalEntry) UpdateExtendData(ctx context.Context, ed types.ExtendData) error {
	const operation = "update_extend_data"
	defer logOperationLatency(entryOperationLatency, operation, time.Now())
	err := i.en.UpdateExtendData(ctx, ed)
	return logOperationError(entryOperationErrorCounter, operation, err)
}

func (i instrumentalEntry) GetExtendField(ctx context.Context, fKey string) (*string, error) {
	const operation = "get_extend_filed"
	defer logOperationLatency(entryOperationLatency, operation, time.Now())
	s, err := i.en.GetExtendField(ctx, fKey)
	return s, logOperationError(entryOperationErrorCounter, operation, err)
}

func (i instrumentalEntry) SetExtendField(ctx context.Context, fKey, fVal string) error {
	const operation = "set_extend_filed"
	defer logOperationLatency(entryOperationLatency, operation, time.Now())
	err := i.en.SetExtendField(ctx, fKey, fVal)
	return logOperationError(entryOperationErrorCounter, operation, err)
}

func (i instrumentalEntry) RemoveExtendField(ctx context.Context, fKey string) error {
	const operation = "remove_extend_filed"
	defer logOperationLatency(entryOperationLatency, operation, time.Now())
	err := i.en.RemoveExtendField(ctx, fKey)
	return logOperationError(entryOperationErrorCounter, operation, err)
}

func (i instrumentalEntry) RuleMatched(ctx context.Context, ruleSpec types.Rule) bool {
	return i.en.RuleMatched(ctx, ruleSpec)
}

func (i instrumentalEntry) IsGroup() bool {
	return i.en.IsGroup()
}

func (i instrumentalEntry) IsMirror() bool {
	return i.en.IsMirror()
}

func (i instrumentalEntry) Group() Group {
	return i.en.Group()
}

type instrumentalFile struct {
	Entry
	file File
}

func (i instrumentalFile) GetAttr() Attr {
	return i.file.GetAttr()
}

func (i instrumentalFile) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	const operation = "write_at"
	defer logOperationLatency(fileOperationLatency, operation, time.Now())
	n, err := i.file.WriteAt(ctx, data, off)
	return n, logOperationError(fileOperationErrorCounter, operation, err)
}

func (i instrumentalFile) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	const operation = "read_at"
	defer logOperationLatency(fileOperationLatency, operation, time.Now())
	n, err := i.file.ReadAt(ctx, dest, off)
	return n, logOperationError(fileOperationErrorCounter, operation, err)
}

func (i instrumentalFile) Fsync(ctx context.Context) error {
	const operation = "fsync"
	defer logOperationLatency(fileOperationLatency, operation, time.Now())
	err := i.file.Fsync(ctx)
	return logOperationError(fileOperationErrorCounter, operation, err)
}

func (i instrumentalFile) Flush(ctx context.Context) error {
	const operation = "flush"
	defer logOperationLatency(fileOperationLatency, operation, time.Now())
	err := i.file.Flush(ctx)
	return logOperationError(fileOperationErrorCounter, operation, err)
}

func (i instrumentalFile) Close(ctx context.Context) error {
	const operation = "close"
	defer logOperationLatency(fileOperationLatency, operation, time.Now())
	err := i.file.Close(ctx)
	return logOperationError(fileOperationErrorCounter, operation, err)
}

type instrumentalGroup struct {
	Entry
	grp Group
}

func (i instrumentalGroup) FindEntry(ctx context.Context, name string) (Entry, error) {
	const operation = "find_entry"
	defer logOperationLatency(groupOperationLatency, operation, time.Now())
	en, err := i.grp.FindEntry(ctx, name)
	return en, logOperationError(groupOperationErrorCounter, operation, err)
}

func (i instrumentalGroup) CreateEntry(ctx context.Context, attr EntryAttr) (Entry, error) {
	const operation = "create_entry"
	defer logOperationLatency(groupOperationLatency, operation, time.Now())
	en, err := i.grp.CreateEntry(ctx, attr)
	return en, logOperationError(groupOperationErrorCounter, operation, err)
}

func (i instrumentalGroup) UpdateEntry(ctx context.Context, en Entry) error {
	const operation = "update_entry"
	defer logOperationLatency(groupOperationLatency, operation, time.Now())
	err := i.grp.UpdateEntry(ctx, en)
	return logOperationError(groupOperationErrorCounter, operation, err)
}

func (i instrumentalGroup) RemoveEntry(ctx context.Context, en Entry) error {
	const operation = "remove_entry"
	defer logOperationLatency(groupOperationLatency, operation, time.Now())
	err := i.grp.RemoveEntry(ctx, en)
	return logOperationError(groupOperationErrorCounter, operation, err)
}

func (i instrumentalGroup) ListChildren(ctx context.Context) ([]Entry, error) {
	const operation = "list_children"
	defer logOperationLatency(groupOperationLatency, operation, time.Now())
	enList, err := i.grp.ListChildren(ctx)
	return enList, logOperationError(groupOperationErrorCounter, operation, err)
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
