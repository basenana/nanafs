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

	"github.com/basenana/nanafs/pkg/types"
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

type instrumentalStore struct {
	store Meta
}

func (i instrumentalStore) SaveEntryUri(ctx context.Context, entryUri *types.EntryUri) error {
	const operation = "save_entry_uri"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SaveEntryUri(ctx, entryUri)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) GetEntryUri(ctx context.Context, uri string) (*types.EntryUri, error) {
	const operation = "get_entry_uri"
	defer logOperationLatency(operation, time.Now())
	en, err := i.store.GetEntryUri(ctx, uri)
	logOperationError(operation, err)
	return en, err
}

func (i instrumentalStore) DeleteEntryUri(ctx context.Context, id int64) error {
	const operation = "delete_entry_uri"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteEntryUri(ctx, id)
	logOperationError(operation, err)
	return err
}

var _ Meta = &instrumentalStore{}

func (i instrumentalStore) SystemInfo(ctx context.Context) (*types.SystemInfo, error) {
	const operation = "system_info"
	defer logOperationLatency(operation, time.Now())
	info, err := i.store.SystemInfo(ctx)
	logOperationError(operation, err)
	return info, err
}

func (i instrumentalStore) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	const operation = "get_entry"
	defer logOperationLatency(operation, time.Now())
	en, err := i.store.GetEntry(ctx, id)
	logOperationError(operation, err)
	return en, err
}

func (i instrumentalStore) FindEntry(ctx context.Context, parentID int64, name string) (*types.Metadata, error) {
	const operation = "find_entry"
	defer logOperationLatency(operation, time.Now())
	en, err := i.store.FindEntry(ctx, parentID, name)
	logOperationError(operation, err)
	return en, err
}

func (i instrumentalStore) CreateEntry(ctx context.Context, parentID int64, newEntry *types.Metadata) error {
	const operation = "create_entry"
	defer logOperationLatency(operation, time.Now())
	err := i.store.CreateEntry(ctx, parentID, newEntry)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) RemoveEntry(ctx context.Context, parentID, entryID int64) error {
	const operation = "remove_entry"
	defer logOperationLatency(operation, time.Now())
	err := i.store.RemoveEntry(ctx, parentID, entryID)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) DeleteRemovedEntry(ctx context.Context, entryID int64) error {
	const operation = "delete_removed_entry"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteRemovedEntry(ctx, entryID)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) UpdateEntryMetadata(ctx context.Context, entry *types.Metadata) error {
	const operation = "update_entry_metadata"
	defer logOperationLatency(operation, time.Now())
	err := i.store.UpdateEntryMetadata(ctx, entry)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) ListEntryChildren(ctx context.Context, parentId int64) (EntryIterator, error) {
	const operation = "list_entry_children"
	defer logOperationLatency(operation, time.Now())
	it, err := i.store.ListEntryChildren(ctx, parentId)
	logOperationError(operation, err)
	return it, err
}

func (i instrumentalStore) FilterEntries(ctx context.Context, filter types.Filter) (EntryIterator, error) {
	const operation = "filter_entries"
	defer logOperationLatency(operation, time.Now())
	it, err := i.store.FilterEntries(ctx, filter)
	logOperationError(operation, err)
	return it, err
}

func (i instrumentalStore) Open(ctx context.Context, id int64, attr types.OpenAttr) (*types.Metadata, error) {
	const operation = "open"
	defer logOperationLatency(operation, time.Now())
	en, err := i.store.Open(ctx, id, attr)
	logOperationError(operation, err)
	return en, err
}

func (i instrumentalStore) Flush(ctx context.Context, id int64, size int64) error {
	const operation = "flush"
	defer logOperationLatency(operation, time.Now())
	err := i.store.Flush(ctx, id, size)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) MirrorEntry(ctx context.Context, newEntry *types.Metadata) error {
	const operation = "mirror_entry"
	defer logOperationLatency(operation, time.Now())
	err := i.store.MirrorEntry(ctx, newEntry)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) ChangeEntryParent(ctx context.Context, targetEntryId int64, newParentId int64, newName string, opt types.ChangeParentAttr) error {
	const operation = "change_entry_parent"
	defer logOperationLatency(operation, time.Now())
	err := i.store.ChangeEntryParent(ctx, targetEntryId, newParentId, newName, opt)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) GetEntryExtendData(ctx context.Context, id int64) (types.ExtendData, error) {
	const operation = "get_entry_extend_data"
	defer logOperationLatency(operation, time.Now())
	ed, err := i.store.GetEntryExtendData(ctx, id)
	logOperationError(operation, err)
	return ed, err
}

func (i instrumentalStore) UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error {
	const operation = "update_entry_extend_data"
	defer logOperationLatency(operation, time.Now())
	err := i.store.UpdateEntryExtendData(ctx, id, ed)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) GetEntryLabels(ctx context.Context, id int64) (types.Labels, error) {
	const operation = "get_entry_labels"
	defer logOperationLatency(operation, time.Now())
	labels, err := i.store.GetEntryLabels(ctx, id)
	logOperationError(operation, err)
	return labels, err
}

func (i instrumentalStore) UpdateEntryLabels(ctx context.Context, id int64, labels types.Labels) error {
	const operation = "update_entry_labels"
	defer logOperationLatency(operation, time.Now())
	err := i.store.UpdateEntryLabels(ctx, id, labels)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) NextSegmentID(ctx context.Context) (int64, error) {
	const operation = "next_segment_id"
	defer logOperationLatency(operation, time.Now())
	segId, err := i.store.NextSegmentID(ctx)
	logOperationError(operation, err)
	return segId, err
}

func (i instrumentalStore) ListSegments(ctx context.Context, oid, chunkID int64, allChunk bool) ([]types.ChunkSeg, error) {
	const operation = "list_segments"
	defer logOperationLatency(operation, time.Now())
	segList, err := i.store.ListSegments(ctx, oid, chunkID, allChunk)
	logOperationError(operation, err)
	return segList, err
}

func (i instrumentalStore) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Metadata, error) {
	const operation = "append_segments"
	defer logOperationLatency(operation, time.Now())
	en, err := i.store.AppendSegments(ctx, seg)
	logOperationError(operation, err)
	return en, err
}

func (i instrumentalStore) DeleteSegment(ctx context.Context, segID int64) error {
	const operation = "delete_segment"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteSegment(ctx, segID)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) ListNotifications(ctx context.Context) ([]types.Notification, error) {
	const operation = "list_notifications"
	defer logOperationLatency(operation, time.Now())
	nList, err := i.store.ListNotifications(ctx)
	logOperationError(operation, err)
	return nList, err
}

func (i instrumentalStore) RecordNotification(ctx context.Context, nid string, no types.Notification) error {
	const operation = "record_notification"
	defer logOperationLatency(operation, time.Now())
	err := i.store.RecordNotification(ctx, nid, no)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) UpdateNotificationStatus(ctx context.Context, nid, status string) error {
	const operation = "update_notification_status"
	defer logOperationLatency(operation, time.Now())
	err := i.store.UpdateNotificationStatus(ctx, nid, status)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) ListTask(ctx context.Context, taskID string, filter types.ScheduledTaskFilter) ([]*types.ScheduledTask, error) {
	const operation = "list_task"
	defer logOperationLatency(operation, time.Now())
	tasks, err := i.store.ListTask(ctx, taskID, filter)
	logOperationError(operation, err)
	return tasks, err
}

func (i instrumentalStore) SaveTask(ctx context.Context, task *types.ScheduledTask) error {
	const operation = "save_task"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SaveTask(ctx, task)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) DeleteFinishedTask(ctx context.Context, aliveTime time.Duration) error {
	const operation = "delete_finished_task"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteFinishedTask(ctx, aliveTime)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) GetWorkflow(ctx context.Context, wfID string) (*types.WorkflowSpec, error) {
	const operation = "get_workflow"
	defer logOperationLatency(operation, time.Now())
	spec, err := i.store.GetWorkflow(ctx, wfID)
	logOperationError(operation, err)
	return spec, err
}

func (i instrumentalStore) ListWorkflow(ctx context.Context) ([]*types.WorkflowSpec, error) {
	const operation = "list_workflow"
	defer logOperationLatency(operation, time.Now())
	specList, err := i.store.ListWorkflow(ctx)
	logOperationError(operation, err)
	return specList, err
}

func (i instrumentalStore) DeleteWorkflow(ctx context.Context, wfID string) error {
	const operation = "delete_workflow"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteWorkflow(ctx, wfID)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) GetWorkflowJob(ctx context.Context, jobID string) (*types.WorkflowJob, error) {
	const operation = "get_workflow_job"
	defer logOperationLatency(operation, time.Now())
	jobList, err := i.store.GetWorkflowJob(ctx, jobID)
	logOperationError(operation, err)
	return jobList, err
}

func (i instrumentalStore) ListWorkflowJob(ctx context.Context, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	const operation = "list_workflow_job"
	defer logOperationLatency(operation, time.Now())
	jobList, err := i.store.ListWorkflowJob(ctx, filter)
	logOperationError(operation, err)
	return jobList, err
}

func (i instrumentalStore) SaveWorkflow(ctx context.Context, wf *types.WorkflowSpec) error {
	const operation = "save_workflow"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SaveWorkflow(ctx, wf)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) SaveWorkflowJob(ctx context.Context, wf *types.WorkflowJob) error {
	const operation = "save_workflow_job"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SaveWorkflowJob(ctx, wf)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) DeleteWorkflowJob(ctx context.Context, wfJobID ...string) error {
	const operation = "delete_workflow_job"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteWorkflowJob(ctx, wfJobID...)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) SaveDocument(ctx context.Context, doc *types.Document) error {
	const operation = "save_document"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SaveDocument(ctx, doc)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) ListDocument(ctx context.Context, parentId int64) ([]*types.Document, error) {
	const operation = "list_document"
	defer logOperationLatency(operation, time.Now())
	docList, err := i.store.ListDocument(ctx, parentId)
	logOperationError(operation, err)
	return docList, err
}

func (i instrumentalStore) GetDocument(ctx context.Context, id int64) (*types.Document, error) {
	const operation = "get_document"
	defer logOperationLatency(operation, time.Now())
	doc, err := i.store.GetDocument(ctx, id)
	logOperationError(operation, err)
	return doc, err
}

func (i instrumentalStore) DeleteDocument(ctx context.Context, id int64) error {
	const operation = "delete_document"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteDocument(ctx, id)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error) {
	const operation = "GetDocumentByEntryId"
	defer logOperationLatency(operation, time.Now())
	doc, err := i.store.GetDocumentByEntryId(ctx, oid)
	logOperationError(operation, err)
	return doc, err
}

func (i instrumentalStore) GetDocumentByName(ctx context.Context, name string) (*types.Document, error) {
	const operation = "GetDocumentByName"
	defer logOperationLatency(operation, time.Now())
	doc, err := i.store.GetDocumentByName(ctx, name)
	logOperationError(operation, err)
	return doc, err
}

func (i instrumentalStore) GetEntryUriById(ctx context.Context, id int64) (*types.EntryUri, error) {
	const operation = "GetEntryUriById"
	defer logOperationLatency(operation, time.Now())
	doc, err := i.store.GetEntryUriById(ctx, id)
	logOperationError(operation, err)
	return doc, err
}

func (i instrumentalStore) DeleteEntryUriByPrefix(ctx context.Context, prefix string) error {
	const operation = "DeleteEntryUriByPrefix"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteEntryUriByPrefix(ctx, prefix)
	logOperationError(operation, err)
	return err
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
