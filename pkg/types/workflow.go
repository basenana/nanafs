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

type WorkflowSpec struct {
	Id              string             `json:"id"`
	Name            string             `json:"name"`
	Rule            Rule               `json:"rule,omitempty"`
	Steps           []WorkflowStepSpec `json:"steps,omitempty"`
	Enable          bool               `json:"enable"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	LastTriggeredAt time.Time          `json:"last_triggered_at"`
}

type WorkflowStepSpec struct {
	Name   string     `json:"name"`
	Plugin *PlugScope `json:"plugin,omitempty"`
}

type WorkflowJob struct {
	Id            string            `json:"id"`
	Workflow      string            `json:"workflow"`
	TriggerReason string            `json:"trigger_reason"`
	Target        WorkflowTarget    `json:"target"`
	Steps         []WorkflowJobStep `json:"steps"`
	Status        string            `json:"status"`
	Message       string            `json:"message"`
	StartAt       time.Time         `json:"start_at"`
	FinishAt      time.Time         `json:"finish_at"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
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
	Message  string     `json:"message"`
	Status   string     `json:"status"`
	Plugin   *PlugScope `json:"plugin,omitempty"`
}

type WorkflowTarget struct {
	EntryID int64 `json:"entry_id"`
}

type WorkflowEntryResult struct {
}
