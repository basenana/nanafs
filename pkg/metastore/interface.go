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
	AccessToken
	SysConfig
	EntryStore
	ChunkStore
	NotificationRecorder
	ScheduledTaskRecorder
}

type AccessToken interface {
	GetAccessToken(ctx context.Context, tokenKey string, secretKey string) (*types.AccessToken, error)
	CreateAccessToken(ctx context.Context, namespace string, token *types.AccessToken) error
	UpdateAccessTokenCerts(ctx context.Context, namespace string, token *types.AccessToken) error
	RevokeAccessToken(ctx context.Context, namespace string, tokenKey string) error
}

type SysConfig interface {
	SystemInfo(ctx context.Context) (*types.SystemInfo, error)
	GetConfigValue(ctx context.Context, namespace, group, name string) (string, error)
	SetConfigValue(ctx context.Context, namespace, group, name, value string) error
}

type EntryStore interface {
	GetEntry(ctx context.Context, namespace string, id int64) (*types.Entry, error)
	CreateEntry(ctx context.Context, namespace string, parentID int64, newEntry *types.Entry, ed *types.ExtendData) error
	DeleteRemovedEntry(ctx context.Context, namespace string, entryID int64) error
	UpdateEntry(ctx context.Context, namespace string, entry *types.Entry) error

	FindEntry(ctx context.Context, namespace string, parentID int64, name string) (*types.Child, error)
	GetChild(ctx context.Context, namespace string, parentID, id int64) (*types.Child, error)
	ListChildren(ctx context.Context, namespace string, parentId int64) ([]*types.Child, error)
	FilterEntries(ctx context.Context, namespace string, filter types.Filter) (EntryIterator, error)
	MirrorEntry(ctx context.Context, namespace string, entryID int64, newName string, newParentID int64) error
	ChangeEntryParent(ctx context.Context, namespace string, targetEntryId int64, oldParentID int64, newParentId int64, oldName string, newName string, opt types.ChangeParentAttr) error
	RemoveEntry(ctx context.Context, namespace string, parentID, entryID int64, entryName string, attr types.DeleteEntry) error

	Open(ctx context.Context, namespace string, id int64, attr types.OpenAttr) (*types.Entry, error)
	Flush(ctx context.Context, namespace string, id int64, size int64) error

	GetEntryExtendData(ctx context.Context, namespace string, id int64) (types.ExtendData, error)
	UpdateEntryExtendData(ctx context.Context, namespace string, id int64, ed types.ExtendData) error

	GetEntryLabels(ctx context.Context, namespace string, id int64) (types.Labels, error)
	UpdateEntryLabels(ctx context.Context, namespace string, id int64, labels types.Labels) error
	ListEntryProperties(ctx context.Context, namespace string, id int64) (types.Properties, error)
	GetEntryProperty(ctx context.Context, namespace string, id int64, key string) (types.PropertyItem, error)
	AddEntryProperty(ctx context.Context, namespace string, id int64, key string, item types.PropertyItem) error
	RemoveEntryProperty(ctx context.Context, namespace string, id int64, key string) error
	UpdateEntryProperties(ctx context.Context, namespace string, id int64, properties types.Properties) error
}

type ChunkStore interface {
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
}

type NotificationRecorder interface {
	ListNotifications(ctx context.Context, namespace string) ([]types.Notification, error)
	RecordNotification(ctx context.Context, namespace string, nid string, no types.Notification) error
	UpdateNotificationStatus(ctx context.Context, namespace, nid, status string) error
}
