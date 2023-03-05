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

type WorkflowSpec struct {
	Id    string             `json:"id"`
	Name  string             `json:"name"`
	Rule  Rule               `json:"rule,omitempty"`
	Steps []WorkflowStepSpec `json:"steps,omitempty"`
}

type WorkflowStepSpec struct {
	Name   string    `json:"name"`
	Plugin PlugScope `json:"plugin"`
}

type WorkflowJob struct {
	Id       string            `json:"id"`
	Workflow string            `json:"workflow"`
	Status   string            `json:"status"`
	Message  string            `json:"message"`
	Steps    []WorkflowJobStep `json:"steps"`
}

type WorkflowJobStep struct {
	StepName string    `json:"step_name"`
	Message  string    `json:"message"`
	Status   string    `json:"status"`
	Plugin   PlugScope `json:"plugin"`
}
