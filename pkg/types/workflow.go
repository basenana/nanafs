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

const (
	WorkflowQueueFile = "file"
)

type Workflow struct {
	Id              string          `json:"id"`
	Namespace       string          `json:"namespace"`
	Name            string          `json:"name"`
	Trigger         WorkflowTrigger `json:"trigger"`
	Nodes           []WorkflowNode  `json:"nodes"`
	Enable          bool            `json:"enable"`
	QueueName       string          `json:"queue_Name"`
	CreatedAt       time.Time       `json:"created_at,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at,omitempty"`
	LastTriggeredAt time.Time       `json:"last_triggered_at,omitempty"`
}

type WorkflowTrigger struct {
	LocalFileWatch *WorkflowLocalFileWatch `json:"local_file_watch,omitempty"`
	RSS            *WorkflowRssTrigger     `json:"rss,omitempty"`
	Interval       *int                    `json:"interval,omitempty"`
}

type WorkflowNode struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Params map[string]string `json:"params"`
	Input  map[string]any    `json:"input,omitempty"`
	Next   string            `json:"next,omitempty"`

	// if
	Condition string            `json:"condition,omitempty"`
	Branches  map[string]string `json:"branches,omitempty"`

	// switch
	Cases   []WorkflowNodeCase `json:"cases,omitempty"`
	Default string             `json:"default,omitempty"`

	// loop
	Matrix *WorkflowNodeMatrix `json:"matrix,omitempty"`
}

type WorkflowNodeCase struct {
	Value string `json:"value"`
	Next  string `json:"next"`
}

type WorkflowLocalFileWatch struct {
	Directory   string `json:"directory"`
	Event       string `json:"event"`
	FilePattern string `json:"file_pattern,omitempty"`
	FileTypes   string `json:"file_types,omitempty"`
	MinFileSize int    `json:"min_file_size,omitempty"`
	MaxFileSize int    `json:"max_file_size,omitempty"`
	CELPattern  string `json:"cel_pattern,omitempty"`
}

type WorkflowRssTrigger struct {
	FileType    string `json:"file_type,omitempty"`
	Timeout     string `json:"timeout,omitempty"`
	ClutterFree bool   `json:"clutter_free,omitempty"`
}

type WorkflowNodeMatrix struct {
	// Data defines variable mappings for matrix iteration
	// Key is variable name, value is template reference to context data
	// Example: {"file_path": "{{ new_webpage.file_paths }}"}
	Data map[string]any `json:"data"`

	// IterateMode controls execution mode: "sequential" (default) or "parallel"
	IterateMode string `json:"iterate_mode,omitempty"`

	// BatchSize limits parallel execution batch size (for parallel mode)
	BatchSize int `json:"batch_size,omitempty"`
}

type WorkflowJob struct {
	Id             string            `json:"id"`
	Namespace      string            `json:"namespace"`
	Workflow       string            `json:"workflow"`
	TriggerReason  string            `json:"trigger_reason,omitempty"`
	Targets        WorkflowTarget    `json:"targets"`
	Nodes          []WorkflowJobNode `json:"nodes"`
	Parameters     map[string]string `json:"parameters"`
	Status         string            `json:"status,omitempty"`
	Message        string            `json:"message,omitempty"`
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

type WorkflowJobNode struct {
	WorkflowNode

	Status     string `json:"status,omitempty"`
	Message    string `json:"message,omitempty"`
	BranchNext string `json:"branch_next,omitempty"`

	// Deprecated
	StepName string `json:"step_name"`
}

type WorkflowTarget struct {
	Entries []string `json:"entries,omitempty"`
}
