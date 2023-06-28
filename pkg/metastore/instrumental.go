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
	"github.com/basenana/nanafs/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"time"
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

func (i instrumentalStore) SystemInfo(ctx context.Context) (*types.SystemInfo, error) {
	const operation = "system_info"
	defer logOperationLatency(operation, time.Now())
	info, err := i.store.SystemInfo(ctx)
	logOperationError(operation, err)
	return info, err
}

func (i instrumentalStore) GetObject(ctx context.Context, id int64) (*types.Object, error) {
	const operation = "get_object"
	defer logOperationLatency(operation, time.Now())
	obj, err := i.store.GetObject(ctx, id)
	logOperationError(operation, err)
	return obj, err
}

func (i instrumentalStore) GetObjectExtendData(ctx context.Context, obj *types.Object) error {
	const operation = "get_object_extend_data"
	defer logOperationLatency(operation, time.Now())
	err := i.store.GetObjectExtendData(ctx, obj)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) ListObjects(ctx context.Context, filter types.Filter) ([]*types.Object, error) {
	const operation = "list_objects"
	defer logOperationLatency(operation, time.Now())
	objList, err := i.store.ListObjects(ctx, filter)
	logOperationError(operation, err)
	return objList, err
}

func (i instrumentalStore) SaveObjects(ctx context.Context, obj ...*types.Object) error {
	const operation = "save_objects"
	defer logOperationLatency(operation, time.Now())
	err := i.store.SaveObjects(ctx, obj...)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) DestroyObject(ctx context.Context, src, obj *types.Object) error {
	const operation = "destroy_object"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DestroyObject(ctx, src, obj)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) ListChildren(ctx context.Context, obj *types.Object) (Iterator, error) {
	const operation = "list_children"
	defer logOperationLatency(operation, time.Now())
	iter, err := i.store.ListChildren(ctx, obj)
	logOperationError(operation, err)
	return iter, err
}

func (i instrumentalStore) MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	const operation = "mirror_object"
	defer logOperationLatency(operation, time.Now())
	err := i.store.MirrorObject(ctx, srcObj, dstParent, object)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) ChangeParent(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	const operation = "change_parent"
	defer logOperationLatency(operation, time.Now())
	err := i.store.ChangeParent(ctx, srcParent, dstParent, obj, opt)
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

func (i instrumentalStore) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Object, error) {
	const operation = "append_segments"
	defer logOperationLatency(operation, time.Now())
	obj, err := i.store.AppendSegments(ctx, seg)
	logOperationError(operation, err)
	return obj, err
}

func (i instrumentalStore) DeleteSegment(ctx context.Context, segID int64) error {
	const operation = "delete_segment"
	defer logOperationLatency(operation, time.Now())
	err := i.store.DeleteSegment(ctx, segID)
	logOperationError(operation, err)
	return err
}

func (i instrumentalStore) PluginRecorder(plugin types.PlugScope) PluginRecorder {
	return i.store.PluginRecorder(plugin)
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

func logOperationLatency(operation string, startAt time.Time) {
	metaOperationLatency.WithLabelValues(operation).Observe(time.Since(startAt).Seconds())
}

func logOperationError(operation string, err error) {
	if err != nil {
		metaOperationErrorCounter.WithLabelValues(operation).Inc()
	}
}
