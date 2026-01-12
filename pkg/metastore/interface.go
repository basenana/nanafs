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

	"github.com/basenana/nanafs/pkg/types"
)

type Meta interface {
	SysConfig
	EntryStore
	NotificationRecorder
	ScheduledTaskRecorder
}

type SysConfig interface {
	SystemInfo(ctx context.Context) (*types.SystemInfo, error)
	GetConfigValue(ctx context.Context, namespace, group, name string) (string, error)
	SetConfigValue(ctx context.Context, namespace, group, name, value string) error
	ListConfigValues(ctx context.Context, namespace, group string) ([]types.ConfigItem, error)
	DeleteConfigValue(ctx context.Context, namespace, group, name string) error
}

type EntryStore interface {
	GetEntry(ctx context.Context, namespace string, id int64) (*types.Entry, error)
	CreateEntry(ctx context.Context, namespace string, parentID int64, newEntry *types.Entry) error
	UpdateEntry(ctx context.Context, namespace string, entry *types.Entry) error

	FindEntry(ctx context.Context, namespace string, parentID int64, name string) (*types.Child, error)
	GetChild(ctx context.Context, namespace string, parentID, id int64) (*types.Child, error)
	ListChildren(ctx context.Context, namespace string, parentId int64) ([]*types.Child, error)
	ListParents(ctx context.Context, namespace string, childID int64) ([]*types.Child, error)
	FilterEntries(ctx context.Context, namespace string, filter types.Filter) (EntryIterator, error)

	MirrorEntry(ctx context.Context, namespace string, entryID int64, newName string, newParentID int64, attr types.EntryAttr) error
	ChangeEntryParent(ctx context.Context, namespace string, targetEntryId int64, oldParentID int64, newParentId int64, oldName string, newName string, opt types.ChangeParentAttr) error
	RemoveEntry(ctx context.Context, namespace string, parentID, entryID int64, entryName string, attr types.DeleteEntry) error
	DeleteRemovedEntry(ctx context.Context, namespace string, entryID int64) error
	ScanOrphanEntries(ctx context.Context, olderThan time.Time) ([]*types.Entry, error)

	Open(ctx context.Context, namespace string, id int64, attr types.OpenAttr) (*types.Entry, error)
	Flush(ctx context.Context, namespace string, id int64, size int64) error

	GetEntryProperties(ctx context.Context, namespace string, ptype types.PropertyType, id int64, data any) error
	UpdateEntryProperties(ctx context.Context, namespace string, ptype types.PropertyType, id int64, data any) error

	NextSegmentID(ctx context.Context) (int64, error)
	ListSegments(ctx context.Context, oid, chunkID int64, allChunk bool) ([]types.ChunkSeg, error)
	AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Entry, error)
	DeleteSegment(ctx context.Context, segID int64) error
}

type ScheduledTaskRecorder interface {
	ListTask(ctx context.Context, taskID string, filter types.ScheduledTaskFilter) ([]*types.ScheduledTask, error)
	SaveTask(ctx context.Context, task *types.ScheduledTask) error
	DeleteFinishedTask(ctx context.Context, aliveTime time.Duration) error

	ListAllNamespaceWorkflows(ctx context.Context) ([]*types.Workflow, error)
	ListAllNamespaceWorkflowJobs(ctx context.Context, filter types.JobFilter) ([]*types.WorkflowJob, error)

	GetWorkflow(ctx context.Context, namespace string, wfID string) (*types.Workflow, error)
	ListWorkflows(ctx context.Context, namespace string) ([]*types.Workflow, error)
	DeleteWorkflow(ctx context.Context, namespace string, wfID string) error
	GetWorkflowJob(ctx context.Context, namespace string, jobID string) (*types.WorkflowJob, error)
	ListWorkflowJobs(ctx context.Context, namespace string, filter types.JobFilter) ([]*types.WorkflowJob, error)
	SaveWorkflow(ctx context.Context, namespace string, wf *types.Workflow) error
	SaveWorkflowJob(ctx context.Context, namespace string, wf *types.WorkflowJob) error
	DeleteWorkflowJobs(ctx context.Context, wfJobID ...string) error

	LoadJobData(ctx context.Context, namespace, source, group, key string, data any) error
	SaveJobData(ctx context.Context, namespace, source, group, key string, data any) error
}

type NotificationRecorder interface {
	ListNotifications(ctx context.Context, namespace string) ([]types.Notification, error)
	RecordNotification(ctx context.Context, namespace string, nid string, no types.Notification) error
	UpdateNotificationStatus(ctx context.Context, namespace, nid, status string) error
}
