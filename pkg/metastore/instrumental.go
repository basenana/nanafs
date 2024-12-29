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

var _ Meta = &instrumentalStore{}

func (i instrumentalStore) GetAccessToken(ctx context.Context, tokenKey string, secretKey string) (*types.AccessToken, error) {
	const operation = "get_access_token"
	defer logOperationLatency(operation, time.Now())
	token, err := i.store.GetAccessToken(ctx, tokenKey, secretKey)
	logOperationError(operation, err)
	return token, err
}

func (i instrumentalStore) CreateAccessToken(ctx context.Context, token *types.AccessToken) error {
	const operation = "create_access_token"
	defer logOperationLatency(operation, time.Now())
	err := i.store.CreateAccessToken(ctx, token)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) UpdateAccessTokenCerts(ctx context.Context, token *types.AccessToken) error {
	const operation = "update_access_token_certs"
	defer logOperationLatency(operation, time.Now())
	err := i.store.UpdateAccessTokenCerts(ctx, token)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) RevokeAccessToken(ctx context.Context, tokenKey string) error {
	const operation = "revoke_access_token"
	defer logOperationLatency(operation, time.Now())
	err := i.store.RevokeAccessToken(ctx, tokenKey)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) SystemInfo(ctx context.Context) (*types.SystemInfo, error) {
	const operation = "system_info"
	defer logOperationLatency(operation, time.Now())
	info, err := i.store.SystemInfo(ctx)
	logOperationError(operation, err)
	return info, err
}

func (i instrumentalStore) GetConfigValue(ctx context.Context, group, name string) (string, error) {
	const operation = "get_config_value"
	defer logOperationLatency(operation, time.Now())
	val, err := i.store.GetConfigValue(ctx, group, name)
	logOperationError(operation, err)
	return val, err
}

func (i instrumentalStore) SetConfigValue(ctx context.Context, group, name, value string) error {
	const operation = "set_config_value"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SetConfigValue(ctx, group, name, value)
	logOperationError(operation, err)
	return err
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

func (i instrumentalStore) CreateEntry(ctx context.Context, parentID int64, newEntry *types.Metadata, ed *types.ExtendData) error {
	const operation = "create_entry"
	defer logOperationLatency(operation, time.Now())
	err := i.store.CreateEntry(ctx, parentID, newEntry, ed)
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

func (i instrumentalStore) ListEntryChildren(ctx context.Context, parentId int64, order *types.EntryOrder, filters ...types.Filter) (EntryIterator, error) {
	const operation = "list_entry_children"
	defer logOperationLatency(operation, time.Now())
	it, err := i.store.ListEntryChildren(ctx, parentId, order, filters...)
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

func (i instrumentalStore) ListEntryProperties(ctx context.Context, id int64) (types.Properties, error) {
	const operation = "list_entry_properties"
	defer logOperationLatency(operation, time.Now())
	result, err := i.store.ListEntryProperties(ctx, id)
	logOperationError(operation, err)
	return result, err
}

func (i instrumentalStore) GetEntryProperty(ctx context.Context, id int64, key string) (types.PropertyItem, error) {
	const operation = "get_entry_property"
	defer logOperationLatency(operation, time.Now())
	result, err := i.store.GetEntryProperty(ctx, id, key)
	logOperationError(operation, err)
	return result, err
}

func (i instrumentalStore) AddEntryProperty(ctx context.Context, id int64, key string, item types.PropertyItem) error {
	const operation = "add_entry_property"
	defer logOperationLatency(operation, time.Now())
	err := i.store.AddEntryProperty(ctx, id, key, item)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) RemoveEntryProperty(ctx context.Context, id int64, key string) error {
	const operation = "remove_entry_property"
	defer logOperationLatency(operation, time.Now())
	err := i.store.RemoveEntryProperty(ctx, id, key)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) UpdateEntryProperties(ctx context.Context, id int64, properties types.Properties) error {
	const operation = "update_entry_properties"
	defer logOperationLatency(operation, time.Now())
	err := i.store.UpdateEntryProperties(ctx, id, properties)
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

func (i instrumentalStore) RecordEvents(ctx context.Context, events []types.Event) error {
	const operation = "record_events"
	defer logOperationLatency(operation, time.Now())
	err := i.store.RecordEvents(ctx, events)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) ListEvents(ctx context.Context, filter types.EventFilter) ([]types.Event, error) {
	const operation = "list_events"
	defer logOperationLatency(operation, time.Now())
	result, err := i.store.ListEvents(ctx, filter)
	logOperationError(operation, err)
	return result, err
}

func (i instrumentalStore) DeviceSync(ctx context.Context, deviceID string, syncedSequence int64) error {
	const operation = "device_sync"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeviceSync(ctx, deviceID, syncedSequence)
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

func (i instrumentalStore) GetWorkflow(ctx context.Context, namespace string, wfID string) (*types.Workflow, error) {
	const operation = "get_workflow"
	defer logOperationLatency(operation, time.Now())
	spec, err := i.store.GetWorkflow(ctx, namespace, wfID)
	logOperationError(operation, err)
	return spec, err
}

func (i instrumentalStore) ListAllNamespaceWorkflows(ctx context.Context) ([]*types.Workflow, error) {
	const operation = "list_all_workflow"
	defer logOperationLatency(operation, time.Now())
	specList, err := i.store.ListAllNamespaceWorkflows(ctx)
	logOperationError(operation, err)
	return specList, err
}

func (i instrumentalStore) ListAllNamespaceWorkflowJobs(ctx context.Context, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	const operation = "list_all_workflow_jobs"
	defer logOperationLatency(operation, time.Now())
	specList, err := i.store.ListAllNamespaceWorkflowJobs(ctx, filter)
	logOperationError(operation, err)
	return specList, err
}

func (i instrumentalStore) ListWorkflows(ctx context.Context, namespace string) ([]*types.Workflow, error) {
	const operation = "list_workflow"
	defer logOperationLatency(operation, time.Now())
	specList, err := i.store.ListWorkflows(ctx, namespace)
	logOperationError(operation, err)
	return specList, err
}

func (i instrumentalStore) DeleteWorkflow(ctx context.Context, namespace string, wfID string) error {
	const operation = "delete_workflow"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteWorkflow(ctx, namespace, wfID)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) GetWorkflowJob(ctx context.Context, namespace string, jobID string) (*types.WorkflowJob, error) {
	const operation = "get_workflow_job"
	defer logOperationLatency(operation, time.Now())
	jobList, err := i.store.GetWorkflowJob(ctx, namespace, jobID)
	logOperationError(operation, err)
	return jobList, err
}

func (i instrumentalStore) ListWorkflowJobs(ctx context.Context, namespace string, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	const operation = "list_workflow_job"
	defer logOperationLatency(operation, time.Now())
	jobList, err := i.store.ListWorkflowJobs(ctx, namespace, filter)
	logOperationError(operation, err)
	return jobList, err
}

func (i instrumentalStore) SaveWorkflow(ctx context.Context, namespace string, wf *types.Workflow) error {
	const operation = "save_workflow"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SaveWorkflow(ctx, namespace, wf)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) SaveWorkflowJob(ctx context.Context, namespace string, wf *types.WorkflowJob) error {
	const operation = "save_workflow_job"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SaveWorkflowJob(ctx, namespace, wf)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) DeleteWorkflowJobs(ctx context.Context, wfJobID ...string) error {
	const operation = "delete_workflow_job"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteWorkflowJobs(ctx, wfJobID...)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) SaveRoom(ctx context.Context, room *types.Room) error {
	const operation = "create_room"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SaveRoom(ctx, room)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) GetRoom(ctx context.Context, id int64) (*types.Room, error) {
	const operation = "get_room"
	defer logOperationLatency(operation, time.Now())
	room, err := i.store.GetRoom(ctx, id)
	logOperationError(operation, err)
	return room, err
}

func (i instrumentalStore) FindRoom(ctx context.Context, entryId int64) (*types.Room, error) {
	const operation = "find_room"
	defer logOperationLatency(operation, time.Now())
	room, err := i.store.FindRoom(ctx, entryId)
	logOperationError(operation, err)
	return room, err
}

func (i instrumentalStore) DeleteRoom(ctx context.Context, id int64) error {
	const operation = "delete_room"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteRoom(ctx, id)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) ListRooms(ctx context.Context, entryId int64) ([]*types.Room, error) {
	const operation = "list_room"
	defer logOperationLatency(operation, time.Now())
	room, err := i.store.ListRooms(ctx, entryId)
	logOperationError(operation, err)
	return room, err
}

func (i instrumentalStore) ListRoomMessage(ctx context.Context, roomId int64) ([]*types.RoomMessage, error) {
	const operation = "list_room_message"
	defer logOperationLatency(operation, time.Now())
	room, err := i.store.ListRoomMessage(ctx, roomId)
	logOperationError(operation, err)
	return room, err
}

func (i instrumentalStore) SaveRoomMessage(ctx context.Context, msg *types.RoomMessage) error {
	const operation = "save_room_message"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SaveRoomMessage(ctx, msg)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) GetRoomMessage(ctx context.Context, msgId int64) (*types.RoomMessage, error) {
	const operation = "get_room_message"
	defer logOperationLatency(operation, time.Now())
	msg, err := i.store.GetRoomMessage(ctx, msgId)
	logOperationError(operation, err)
	return msg, err
}

func (i instrumentalStore) DeleteRoomMessages(ctx context.Context, roomId int64) error {
	const operation = "delete_room_message"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteRoomMessages(ctx, roomId)
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
