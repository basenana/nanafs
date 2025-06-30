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

type SystemConfig struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Group     string    `gorm:"column:cfg_group;index:cfg_group"`
	Name      string    `gorm:"column:cfg_name;index:cfg_name"`
	Value     string    `gorm:"column:cfg_value"`
	Namespace string    `gorm:"column:namespace;index:cfg_namespace"`
	ChangedAt time.Time `gorm:"column:changed_at"`
}

func (i SystemConfig) TableName() string {
	return "system_config"
}

type Entry struct {
	ID         int64   `gorm:"column:id;primaryKey"`
	Name       string  `gorm:"column:name;index:en_name"`
	Aliases    *string `gorm:"column:aliases"`
	RefCount   *int    `gorm:"column:ref_count"`
	Kind       string  `gorm:"column:kind"`
	IsGroup    bool    `gorm:"column:is_group;index:en_isgrp"`
	Size       *int64  `gorm:"column:size"`
	Version    int64   `gorm:"column:version;index:en_version"`
	Owner      *int64  `gorm:"column:owner"`
	GroupOwner *int64  `gorm:"column:group_owner"`
	Permission *int64  `gorm:"column:permission"`
	Storage    string  `gorm:"column:storage"`
	Namespace  string  `gorm:"column:namespace;index:en_ns"`
	CreatedAt  int64   `gorm:"column:created_at"`
	ChangedAt  int64   `gorm:"column:changed_at"`
	ModifiedAt int64   `gorm:"column:modified_at"`
	AccessAt   int64   `gorm:"column:access_at"`
}

func (o *Entry) TableName() string {
	return "entry"
}

func (o *Entry) FromEntry(en *types.Entry) *Entry {
	o.ID = en.ID
	o.Name = en.Name
	o.Aliases = &en.Aliases
	o.RefCount = &en.RefCount
	o.Kind = string(en.Kind)
	o.IsGroup = en.IsGroup
	o.Size = &en.Size
	o.Version = en.Version
	o.Storage = en.Storage
	o.Namespace = en.Namespace
	o.CreatedAt = en.CreatedAt.UnixNano()
	o.ChangedAt = en.ChangedAt.UnixNano()
	o.ModifiedAt = en.ModifiedAt.UnixNano()
	o.AccessAt = en.AccessAt.UnixNano()
	o.Owner = &en.Access.UID
	o.GroupOwner = &en.Access.GID
	o.Permission = updateEntryPermission(en.Access)
	return o
}

func (o *Entry) ToEntry() *types.Entry {
	result := &types.Entry{
		ID:         o.ID,
		Name:       o.Name,
		Kind:       types.Kind(o.Kind),
		IsGroup:    o.IsGroup,
		Version:    o.Version,
		Storage:    o.Storage,
		Namespace:  o.Namespace,
		CreatedAt:  time.Unix(0, o.CreatedAt),
		ChangedAt:  time.Unix(0, o.ChangedAt),
		ModifiedAt: time.Unix(0, o.ModifiedAt),
		AccessAt:   time.Unix(0, o.AccessAt),
		Access:     buildEntryAccess(o.Permission, o.Owner, o.GroupOwner),
	}
	if o.Aliases != nil {
		result.Aliases = *o.Aliases
	}
	if o.RefCount != nil {
		result.RefCount = *o.RefCount
	}
	if o.Size != nil {
		result.Size = *o.Size
	}
	return result
}

type Children struct {
	ParentID  int64  `gorm:"column:parent_id;primaryKey"`
	ChildID   int64  `gorm:"column:child_id;primaryKey"`
	Name      string `gorm:"column:name;primaryKey"`
	Namespace string `gorm:"column:namespace;index:child_ns"`
	Dynamic   bool   `gorm:"column:dynamic;index:child_dy"`
}

func (c *Children) From(child *types.Child) {
	c.ParentID = child.ParentID
	c.ChildID = child.ChildID
	c.Name = child.Name
	c.Namespace = child.Namespace
	c.Dynamic = child.Dynamic
}

func (c *Children) To() *types.Child {
	return &types.Child{
		ParentID:  c.ParentID,
		ChildID:   c.ChildID,
		Name:      c.Name,
		Namespace: c.Namespace,
		Dynamic:   c.Dynamic,
	}
}

func (c *Children) TableName() string {
	return "children"
}

type EntryProperty struct {
	Entry     int64  `gorm:"column:entry;primaryKey"`
	Type      string `gorm:"column:type;primaryKey"`
	Value     JSON   `gorm:"column:value;index:prop_val"`
	Namespace string `gorm:"column:namespace;index:prop_ns"`
}

func (o EntryProperty) TableName() string {
	return "entry_property"
}

type EntryChunk struct {
	ID       int64 `gorm:"column:id;primaryKey"`
	Entry    int64 `gorm:"column:entry;index:ck_eid"`
	ChunkID  int64 `gorm:"column:chunk_id;index:ck_id"`
	Off      int64 `gorm:"column:off"`
	Len      int64 `gorm:"column:len"`
	State    int16 `gorm:"column:state"`
	AppendAt int64 `gorm:"column:append_at;index:ck_append_at"`
}

func (o EntryChunk) TableName() string {
	return "entry_chunk"
}

type Project struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Goal      string    `gorm:"column:goal"`
	Details   string    `gorm:"column:details"`
	Config    JSON      `gorm:"column:config"`
	Paused    bool      `gorm:"column:paused;index:proj_paused"`
	Archived  bool      `gorm:"column:archived;index:proj_arched"`
	CreatedAt time.Time `gorm:"column:created_at;index:proj_creat"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (p Project) TableName() string {
	return "project"
}

type ScheduledTask struct {
	ID             int64     `gorm:"column:id;autoIncrement"`
	TaskID         string    `gorm:"column:task_id;index:st_task_id"`
	Namespace      string    `gorm:"column:namespace;index:st_task_ns"`
	RefType        string    `gorm:"column:ref_type;index:sche_task_reftype"`
	RefID          int64     `gorm:"column:ref_id;index:sche_task_refid"`
	Status         string    `gorm:"column:status;index:st_task_status"`
	Result         string    `gorm:"column:result"`
	CreatedTime    time.Time `gorm:"column:created_time"`
	ExecutionTime  time.Time `gorm:"column:execution_time"`
	ExpirationTime time.Time `gorm:"column:expiration_time"`
	Event          JSON      `gorm:"column:event"`
}

func (d ScheduledTask) TableName() string {
	return "scheduled_task"
}

type Notification struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Title     string    `gorm:"column:title"`
	Message   string    `gorm:"column:message"`
	Namespace string    `gorm:"column:namespace;index:notif_ns"`
	Type      string    `gorm:"column:type"`
	Source    string    `gorm:"column:source"`
	Action    string    `gorm:"column:action"`
	Status    string    `gorm:"column:status"`
	Time      time.Time `gorm:"column:time;index:notif_time"`
}

func (o *Notification) TableName() string {
	return "notification"
}

type Workflow struct {
	ID              string    `gorm:"column:id;primaryKey"`
	Name            string    `gorm:"column:name"`
	Rule            JSON      `gorm:"column:rule"`
	Cron            string    `gorm:"column:cron"`
	Steps           JSON      `gorm:"column:steps"`
	Enable          bool      `gorm:"column:enable;index:wf_enable"`
	QueueName       string    `gorm:"column:queue_name;index:wf_q"`
	Namespace       string    `gorm:"column:namespace;index:wf_ns"`
	CreatedAt       time.Time `gorm:"column:created_at;index:wf_creat"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
	LastTriggeredAt time.Time `gorm:"column:last_triggered_at"`
}

func (o *Workflow) TableName() string {
	return "workflow"
}

func (o *Workflow) From(wf *types.Workflow) (*Workflow, error) {
	o.ID = wf.Id
	o.Name = wf.Name
	o.Namespace = wf.Namespace
	o.Enable = wf.Enable
	o.Cron = wf.Cron
	o.QueueName = wf.QueueName
	o.CreatedAt = wf.CreatedAt
	o.UpdatedAt = wf.UpdatedAt
	o.LastTriggeredAt = wf.LastTriggeredAt

	if wf.Rule != nil {
		rawRule, err := json.Marshal(wf.Rule)
		if err != nil {
			return o, fmt.Errorf("marshal workflow rule config failed: %s", err)
		}
		o.Rule = rawRule
	}

	rawSteps, err := json.Marshal(wf.Steps)
	if err != nil {
		return o, fmt.Errorf("marshal workflow rule config failed: %s", err)
	}
	o.Steps = rawSteps
	return o, nil
}

func (o *Workflow) To() (*types.Workflow, error) {
	result := &types.Workflow{
		Id:              o.ID,
		Name:            o.Name,
		Namespace:       o.Namespace,
		Enable:          o.Enable,
		Cron:            o.Cron,
		Steps:           []types.WorkflowStepSpec{},
		QueueName:       o.QueueName,
		CreatedAt:       o.CreatedAt,
		UpdatedAt:       o.UpdatedAt,
		LastTriggeredAt: o.LastTriggeredAt,
	}

	if len(o.Rule) > 0 {
		if err := json.Unmarshal(o.Rule, &result.Rule); err != nil {
			return nil, fmt.Errorf("load workflow rule config failed: %s", err)
		}
	}

	if err := json.Unmarshal(o.Steps, &result.Steps); err != nil {
		return nil, fmt.Errorf("load workflow steps config failed: %s", err)
	}
	return result, nil
}

type WorkflowJob struct {
	ID            string    `gorm:"column:id;autoIncrement"`
	Workflow      string    `gorm:"column:workflow;index:job_wf_id"`
	TriggerReason string    `gorm:"column:trigger_reason"`
	Target        JSON      `gorm:"column:target"`
	Steps         JSON      `gorm:"column:steps"`
	Parameters    JSON      `gorm:"column:parameters"`
	Status        string    `gorm:"column:status;index:job_status"`
	Message       string    `gorm:"column:message"`
	QueueName     string    `gorm:"column:queue_name;index:job_queue"`
	Executor      string    `gorm:"column:executor;index:job_executor"`
	Namespace     string    `gorm:"column:namespace;index:job_ns"`
	StartAt       time.Time `gorm:"column:start_at"`
	FinishAt      time.Time `gorm:"column:finish_at"`
	CreatedAt     time.Time `gorm:"column:created_at;index:job_created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (o *WorkflowJob) TableName() string {
	return "workflow_job"
}

func (o *WorkflowJob) From(job *types.WorkflowJob) (*WorkflowJob, error) {
	o.ID = job.Id
	o.Workflow = job.Workflow
	o.TriggerReason = job.TriggerReason
	o.Status = job.Status
	o.Message = job.Message
	o.QueueName = job.QueueName
	o.Executor = job.Executor
	o.StartAt = job.StartAt
	o.FinishAt = job.FinishAt
	o.CreatedAt = job.CreatedAt
	o.UpdatedAt = job.UpdatedAt
	o.Namespace = job.Namespace

	rawTarget, err := json.Marshal(job.Target)
	if err != nil {
		return o, fmt.Errorf("marshal workflow job target failed: %s", err)
	}
	o.Target = rawTarget

	rawPara, err := json.Marshal(job.Parameters)
	if err != nil {
		return o, fmt.Errorf("marshal workflow job steps failed: %s", err)
	}
	o.Parameters = rawPara

	rawStep, err := json.Marshal(job.Steps)
	if err != nil {
		return o, fmt.Errorf("marshal workflow job steps failed: %s", err)
	}
	o.Steps = rawStep

	return o, nil
}

func (o *WorkflowJob) To() (*types.WorkflowJob, error) {
	result := &types.WorkflowJob{
		Id:            o.ID,
		Workflow:      o.Workflow,
		TriggerReason: o.TriggerReason,
		Steps:         []types.WorkflowJobStep{},
		Status:        o.Status,
		Message:       o.Message,
		Executor:      o.Executor,
		QueueName:     o.QueueName,
		StartAt:       o.StartAt,
		FinishAt:      o.FinishAt,
		CreatedAt:     o.CreatedAt,
		UpdatedAt:     o.UpdatedAt,
		Namespace:     o.Namespace,
	}

	err := json.Unmarshal(o.Target, &result.Target)
	if err != nil {
		return nil, fmt.Errorf("load workflow job target failed: %s", err)
	}

	err = json.Unmarshal(o.Parameters, &result.Parameters)
	if err != nil {
		return nil, fmt.Errorf("load workflow job steps failed: %s", err)
	}

	err = json.Unmarshal(o.Steps, &result.Steps)
	if err != nil {
		return nil, fmt.Errorf("load workflow job steps failed: %s", err)
	}

	return result, nil
}
