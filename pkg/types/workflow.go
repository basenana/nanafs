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

package types

import (
	"time"
)

type Workflow struct {
	Id              string             `json:"id"`
	Name            string             `json:"name"`
	Namespace       string             `json:"namespace"`
	Rule            Rule               `json:"rule,omitempty"`
	Cron            string             `json:"cron,omitempty"`
	Steps           []WorkflowStepSpec `json:"steps,omitempty"`
	Enable          bool               `json:"enable"`
	Executor        string             `json:"executor"`
	QueueName       string             `json:"queue_name"`
	HealthScore     int                `json:"health_score,omitempty"`
	CreatedAt       time.Time          `json:"created_at,omitempty"`
	UpdatedAt       time.Time          `json:"updated_at,omitempty"`
	LastTriggeredAt time.Time          `json:"last_triggered_at,omitempty"`
}

type WorkflowStepSpec struct {
	Name   string     `json:"name"`
	Plugin *PlugScope `json:"plugin,omitempty"`
}

type WorkflowJob struct {
	Id             string            `json:"id"`
	Namespace      string            `json:"namespace"`
	Workflow       string            `json:"workflow"`
	TriggerReason  string            `json:"trigger_reason,omitempty"`
	Target         WorkflowTarget    `json:"target"`
	Steps          []WorkflowJobStep `json:"steps"`
	Status         string            `json:"status,omitempty"`
	Message        string            `json:"message,omitempty"`
	Executor       string            `json:"executor"`
	QueueName      string            `json:"queue_name"`
	TimeoutSeconds int               `json:"timeout"`
	StartAt        time.Time         `json:"start_at"`
	FinishAt       time.Time         `json:"finish_at"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func (w *WorkflowJob) GetStatus() string {
	return w.Status
}

func (w *WorkflowJob) SetStatus(status string) {
	w.Status = status
}

func (w *WorkflowJob) GetMessage() string {
	return w.Message
}

func (w *WorkflowJob) SetMessage(msg string) {
	w.Message = msg
}

type WorkflowJobStep struct {
	StepName string     `json:"step_name"`
	Message  string     `json:"message,omitempty"`
	Status   string     `json:"status,omitempty"`
	Plugin   *PlugScope `json:"plugin,omitempty"`
}

type WorkflowTarget struct {
	EntryID       int64 `json:"entry_id,omitempty"`
	ParentEntryID int64 `json:"parent_entry_id,omitempty"`
}

type WorkflowEntryResult struct {
}
