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

package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/basenana/nanafs/pkg/types"
)

type SystemInfo struct {
	FsID     string `gorm:"column:fs_id;primaryKey"`
	ChunkSeg int64  `gorm:"column:chunk_seg"`
}

func (i SystemInfo) TableName() string {
	return "system_info"
}

type Object struct {
	ID         int64   `gorm:"column:id;primaryKey"`
	Name       string  `gorm:"column:name;index:obj_name"`
	Aliases    *string `gorm:"column:aliases"`
	ParentID   *int64  `gorm:"column:parent_id;index:parent_id"`
	RefID      *int64  `gorm:"column:ref_id;index:ref_id"`
	RefCount   *int    `gorm:"column:ref_count"`
	Kind       string  `gorm:"column:kind"`
	KindMap    *int64  `gorm:"column:kind_map"`
	Size       *int64  `gorm:"column:size"`
	Version    int64   `gorm:"column:version;index:obj_version"`
	Dev        int64   `gorm:"column:dev"`
	Owner      *int64  `gorm:"column:owner"`
	GroupOwner *int64  `gorm:"column:group_owner"`
	Permission *int64  `gorm:"column:permission"`
	Storage    string  `gorm:"column:storage"`
	Namespace  string  `gorm:"column:namespace;index:obj_ns"`
	CreatedAt  int64   `gorm:"column:created_at"`
	ChangedAt  int64   `gorm:"column:changed_at"`
	ModifiedAt int64   `gorm:"column:modified_at"`
	AccessAt   int64   `gorm:"column:access_at"`
}

func (o *Object) TableName() string {
	return "object"
}

func (o *Object) Update(obj *types.Object) {
	o.ID = obj.ID
	o.Name = obj.Name
	o.Aliases = &obj.Aliases
	o.ParentID = &obj.ParentID
	o.RefID = &obj.RefID
	o.RefCount = &obj.RefCount
	o.Kind = string(obj.Kind)
	o.KindMap = &obj.KindMap
	o.Size = &obj.Size
	o.Version = obj.Version
	o.Dev = obj.Dev
	o.Storage = obj.Storage
	o.Namespace = obj.Namespace
	o.CreatedAt = obj.CreatedAt.UnixNano()
	o.ChangedAt = obj.ChangedAt.UnixNano()
	o.ModifiedAt = obj.ModifiedAt.UnixNano()
	o.AccessAt = obj.AccessAt.UnixNano()
	o.Owner = &obj.Access.UID
	o.GroupOwner = &obj.Access.GID
	o.Permission = updateObjectPermission(obj.Access)
}

func (o *Object) Object() *types.Object {
	result := &types.Object{
		Metadata: types.Metadata{
			ID:         o.ID,
			Name:       o.Name,
			Kind:       types.Kind(o.Kind),
			Version:    o.Version,
			Dev:        o.Dev,
			Storage:    o.Storage,
			Namespace:  o.Namespace,
			CreatedAt:  time.Unix(0, o.CreatedAt),
			ChangedAt:  time.Unix(0, o.ChangedAt),
			ModifiedAt: time.Unix(0, o.ModifiedAt),
			AccessAt:   time.Unix(0, o.AccessAt),
			Access:     buildObjectAccess(o.Permission, o.Owner, o.GroupOwner),
		},
	}

	if o.Aliases != nil {
		result.Aliases = *o.Aliases
	}
	if o.ParentID != nil {
		result.ParentID = *o.ParentID
	}
	if o.RefID != nil {
		result.RefID = *o.RefID
	}
	if o.RefCount != nil {
		result.RefCount = *o.RefCount
	}
	if o.KindMap != nil {
		result.KindMap = *o.KindMap
	}
	if o.Size != nil {
		result.Size = *o.Size
	}
	return result
}

type Label struct {
	ID        int64  `gorm:"column:id;autoIncrement"`
	RefID     int64  `gorm:"column:ref_id;index:label_refid"`
	RefType   string `gorm:"column:ref_type;index:label_reftype"`
	Key       string `gorm:"column:key"`
	Value     string `gorm:"column:value"`
	SearchKey string `gorm:"column:search_key;index:label_search_key"`
}

func (o Label) TableName() string {
	return "label"
}

type ObjectProperty struct {
	ID    int64  `gorm:"column:id;autoIncrement"`
	OID   int64  `gorm:"column:oid;index:prop_oid"`
	Name  string `gorm:"column:key;index:prop_name"`
	Value string `gorm:"column:value"`
}

func (o ObjectProperty) TableName() string {
	return "object_property"
}

type ObjectExtend struct {
	ID          int64  `gorm:"column:id;primaryKey"`
	Symlink     string `gorm:"column:symlink"`
	GroupFilter []byte `gorm:"column:group_filter"`
	PlugScope   []byte `gorm:"column:plug_scope"`
}

func (o *ObjectExtend) TableName() string {
	return "object_extend"
}

func (o *ObjectExtend) Update(obj *types.Object) {
	if obj.ExtendData != nil {
		o.Symlink = obj.ExtendData.Symlink
		if obj.ExtendData.GroupFilter != nil {
			o.GroupFilter, _ = json.Marshal(obj.ExtendData.GroupFilter)
		}
		if obj.ExtendData.PlugScope != nil {
			o.PlugScope, _ = json.Marshal(obj.ExtendData.PlugScope)
		}
	}
}

func (o *ObjectExtend) ToExtData() types.ExtendData {
	ext := types.ExtendData{
		Properties:  types.Properties{Fields: map[string]string{}},
		Symlink:     o.Symlink,
		GroupFilter: nil,
		PlugScope:   nil,
	}
	if o.GroupFilter != nil {
		_ = json.Unmarshal(o.GroupFilter, &ext.GroupFilter)
	}
	if o.PlugScope != nil {
		_ = json.Unmarshal(o.PlugScope, &ext.PlugScope)
	}
	return ext
}

type ObjectChunk struct {
	ID       int64 `gorm:"column:id;primaryKey"`
	OID      int64 `gorm:"column:oid;index:ck_oid"`
	ChunkID  int64 `gorm:"column:chunk_id;index:ck_id"`
	Off      int64 `gorm:"column:off"`
	Len      int64 `gorm:"column:len"`
	State    int16 `gorm:"column:state"`
	AppendAt int64 `gorm:"column:append_at;index:ck_append_at"`
}

func (o ObjectChunk) TableName() string {
	return "object_chunk"
}

type ScheduledTask struct {
	ID             int64     `gorm:"column:id;autoIncrement"`
	TaskID         string    `gorm:"column:task_id;index:st_task_id"`
	RefType        string    `gorm:"column:ref_type;index:sche_task_reftype"`
	RefID          int64     `gorm:"column:ref_id;index:sche_task_refid"`
	Status         string    `gorm:"column:status;index:st_task_status"`
	Result         string    `gorm:"column:result"`
	CreatedTime    time.Time `gorm:"column:created_time"`
	ExecutionTime  time.Time `gorm:"column:execution_time"`
	ExpirationTime time.Time `gorm:"column:expiration_time"`
	Event          string    `gorm:"column:event"`
}

func (d ScheduledTask) TableName() string {
	return "scheduled_task"
}

type PluginData struct {
	ID         int64            `gorm:"column:id;autoIncrement"`
	PluginName string           `gorm:"column:plugin_name;index:plugin_name"`
	Version    string           `gorm:"column:version"`
	Type       types.PluginType `gorm:"column:type"`
	GroupId    string           `gorm:"column:group_id;index:group_id"`
	RecordId   string           `gorm:"column:record_id;index:record_id"`
	Content    string           `gorm:"column:content"`
}

func (d PluginData) TableName() string {
	return "plugin_data"
}

type Notification struct {
	ID      string    `gorm:"column:id;primaryKey"`
	Title   string    `gorm:"column:title"`
	Message string    `gorm:"column:message"`
	Type    string    `gorm:"column:type"`
	Source  string    `gorm:"column:source"`
	Action  string    `gorm:"column:action"`
	Status  string    `gorm:"column:status"`
	Time    time.Time `gorm:"column:time;index:notif_time"`
}

func (o *Notification) TableName() string {
	return "notification"
}

type Workflow struct {
	ID              string    `gorm:"column:id;primaryKey"`
	Name            string    `gorm:"column:name"`
	Rule            string    `gorm:"column:rule"`
	Steps           string    `gorm:"column:steps"`
	Enable          bool      `gorm:"column:enable;index:wf_enable"`
	CreatedAt       time.Time `gorm:"column:created_at;index:wf_creat"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
	LastTriggeredAt time.Time `gorm:"column:last_triggered_at"`
}

func (o *Workflow) TableName() string {
	return "workflow"
}

func (o *Workflow) Update(wf *types.WorkflowSpec) error {
	o.ID = wf.Id
	o.Name = wf.Name
	o.Enable = wf.Enable
	o.CreatedAt = wf.CreatedAt
	o.UpdatedAt = wf.UpdatedAt
	o.LastTriggeredAt = wf.LastTriggeredAt

	rawRule, err := json.Marshal(wf.Rule)
	if err != nil {
		return fmt.Errorf("marshal workflow rule config failed: %s", err)
	}
	o.Rule = string(rawRule)

	rawSteps, err := json.Marshal(wf.Steps)
	if err != nil {
		return fmt.Errorf("marshal workflow rule config failed: %s", err)
	}
	o.Steps = string(rawSteps)
	return nil
}

func (o *Workflow) ToWorkflowSpec() (*types.WorkflowSpec, error) {
	result := &types.WorkflowSpec{
		Id:              o.ID,
		Name:            o.Name,
		Enable:          o.Enable,
		Steps:           []types.WorkflowStepSpec{},
		CreatedAt:       o.CreatedAt,
		UpdatedAt:       o.UpdatedAt,
		LastTriggeredAt: o.LastTriggeredAt,
	}

	if err := json.Unmarshal([]byte(o.Rule), &result.Rule); err != nil {
		return nil, fmt.Errorf("load workflow rule config failed: %s", err)
	}
	if err := json.Unmarshal([]byte(o.Steps), &result.Steps); err != nil {
		return nil, fmt.Errorf("load workflow steps config failed: %s", err)
	}
	return result, nil
}

type WorkflowJob struct {
	ID            string    `gorm:"column:id;autoIncrement"`
	Workflow      string    `gorm:"column:workflow;index:job_wf_id"`
	TriggerReason string    `gorm:"column:trigger_reason"`
	TargetEntry   int64     `gorm:"column:target_entry;index:job_tgt_en"`
	Target        string    `gorm:"column:target"`
	Steps         string    `gorm:"column:steps"`
	Status        string    `gorm:"column:status;index:job_status"`
	Message       string    `gorm:"column:message"`
	Executor      string    `gorm:"column:executor;index:job_executor"`
	StartAt       time.Time `gorm:"column:start_at"`
	FinishAt      time.Time `gorm:"column:finish_at"`
	CreatedAt     time.Time `gorm:"column:created_at;index:job_created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (o *WorkflowJob) TableName() string {
	return "workflow_job"
}

func (o *WorkflowJob) Update(job *types.WorkflowJob) error {
	o.ID = job.Id
	o.Workflow = job.Workflow
	o.TriggerReason = job.TriggerReason
	o.Status = job.Status
	o.Message = job.Message
	o.Executor = "any"
	o.StartAt = job.StartAt
	o.FinishAt = job.FinishAt
	o.CreatedAt = job.CreatedAt
	o.UpdatedAt = job.UpdatedAt

	rawTarget, err := json.Marshal(job.Target)
	if err != nil {
		return fmt.Errorf("marshal workflow job target failed: %s", err)
	}
	o.TargetEntry = job.Target.EntryID
	o.Target = string(rawTarget)

	rawStep, err := json.Marshal(job.Steps)
	if err != nil {
		return fmt.Errorf("marshal workflow job steps failed: %s", err)
	}
	o.Steps = string(rawStep)

	return nil
}

func (o *WorkflowJob) ToWorkflowJobSpec() (*types.WorkflowJob, error) {
	result := &types.WorkflowJob{
		Id:            o.ID,
		Workflow:      o.Workflow,
		TriggerReason: o.TriggerReason,
		Steps:         []types.WorkflowJobStep{},
		Status:        o.Status,
		Message:       o.Message,
		StartAt:       o.StartAt,
		FinishAt:      o.FinishAt,
		CreatedAt:     o.CreatedAt,
		UpdatedAt:     o.UpdatedAt,
	}

	err := json.Unmarshal([]byte(o.Target), &result.Target)
	if err != nil {
		return nil, fmt.Errorf("load workflow job target failed: %s", err)
	}

	err = json.Unmarshal([]byte(o.Steps), &result.Steps)
	if err != nil {
		return nil, fmt.Errorf("load workflow job steps failed: %s", err)
	}

	return result, nil
}

type Document struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Name      string    `gorm:"column:name;index:name"`
	Source    string    `gorm:"column:source;index:source"`
	Uri       string    `gorm:"column:uri;index:uri"`
	KeyWords  string    `gorm:"column:keywords"`
	Content   string    `gorm:"column:content"`
	Summary   string    `gorm:"column:summary"`
	CreatedAt time.Time `gorm:"column:created_at"`
	ChangedAt time.Time `gorm:"column:changed_at"`
}

func (d *Document) TableName() string {
	return "document"
}

func (d *Document) Update(document *types.Document) error {
	d.ID = document.ID
	d.Name = document.Name
	d.Uri = document.Uri
	d.KeyWords = strings.Join(document.KeyWords, ",")
	d.Source = document.Source
	d.Content = document.Content
	d.Summary = document.Summary
	d.CreatedAt = document.CreatedAt
	d.ChangedAt = document.ChangedAt
	return nil
}

func (d *Document) ToDocument() (*types.Document, error) {
	result := &types.Document{
		ID:        d.ID,
		Name:      d.Name,
		Uri:       d.Uri,
		Source:    d.Source,
		KeyWords:  strings.Split(d.KeyWords, ","),
		Content:   d.Content,
		Summary:   d.Summary,
		CreatedAt: d.CreatedAt,
		ChangedAt: d.ChangedAt,
	}
	return result, nil
}
