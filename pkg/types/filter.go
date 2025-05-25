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

import "time"

type Filter struct {
	ID              int64
	Name            string
	FuzzyName       string
	ParentID        int64
	Kind            Kind
	Label           LabelMatch
	IsGroup         *bool
	CreatedAtStart  *time.Time
	CreatedAtEnd    *time.Time
	ModifiedAtStart *time.Time
	ModifiedAtEnd   *time.Time
}

type JobFilter struct {
	WorkFlowID  string
	JobID       string
	Status      string
	QueueName   string
	Executor    string
	TargetEntry int64
}

type LabelMatch struct {
	Include []Label  `json:"include"`
	Exclude []string `json:"exclude"`
}

type EventFilter struct {
	StartSequence int64
	Limit         int
	DESC          bool
}
