/*
   Copyright 2023 Go-Flow Authors

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

package flow

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"
)

type testExecutor struct{}

func (t testExecutor) Setup(ctx context.Context) error {
	return nil
}

func (t testExecutor) DoOperation(ctx context.Context, task Task, operatorSpec Spec) error {
	if operatorSpec.Script != nil && len(operatorSpec.Script.Command) > 1 {
		if operatorSpec.Script.Command[0] == "echo" && operatorSpec.Script.Command[1] == "failed" {
			return fmt.Errorf("echo failed")
		}

		if operatorSpec.Script.Command[0] == "sleep" {
			sTime, _ := strconv.Atoi(operatorSpec.Script.Command[1])
			time.Sleep(time.Duration(sTime) * time.Second)
		}
	}
	return nil
}

func (t testExecutor) Teardown(ctx context.Context) {}

func init() {
	RegisterExecutorBuilder("test", func(flow *Flow) Executor {
		return testExecutor{}
	})
}

func Test_runner_Start(t *testing.T) {
	type fields struct {
		flow    *Flow
		storage Storage
	}
	tests := []struct {
		name        string
		fields      fields
		finalStatus string
	}{
		{
			name: "test-simple-flow-1",
			fields: fields{
				flow: &Flow{
					ID:            "test-simple-flow-1",
					Executor:      "test",
					ControlPolicy: ControlPolicy{FailedPolicy: PolicyFastFailed},
					Tasks: []Task{
						{
							Name:         "test-1",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-1"}}},
						},
					},
				},
				storage: NewInMemoryStorage(),
			},
			finalStatus: SucceedStatus,
		},
		{
			name: "test-simple-flow-2",
			fields: fields{
				flow: &Flow{
					ID:            "test-simple-flow-2",
					Executor:      "test",
					ControlPolicy: ControlPolicy{FailedPolicy: PolicyFastFailed},
					Tasks: []Task{
						{
							Name:         "test-1",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "failed"}}},
						},
					},
				},
				storage: NewInMemoryStorage(),
			},
			finalStatus: FailedStatus,
		},
		{
			name: "test-pipeline-flow-1",
			fields: fields{
				flow: &Flow{
					ID:            "test-pipeline-flow-1",
					Executor:      "test",
					ControlPolicy: ControlPolicy{FailedPolicy: PolicyFastFailed},
					Tasks: []Task{
						{
							Name:         "test-1",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-1"}}},
						},
						{
							Name:         "test-2",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-2"}}},
						},
						{
							Name:         "test-3",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-3"}}},
						},
					},
				},
				storage: NewInMemoryStorage(),
			},
			finalStatus: SucceedStatus,
		},
		{
			name: "test-graph-flow-1",
			fields: fields{
				flow: &Flow{
					ID:            "test-pipeline-flow-1",
					Executor:      "test",
					ControlPolicy: ControlPolicy{FailedPolicy: PolicyFastFailed},
					Tasks: []Task{
						{
							Name:         "test-1",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-1"}}},
							Next:         NextTask{OnSucceed: "test-2", OnFailed: "test-3"},
						},
						{
							Name:         "test-2",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-2"}}},
						},
						{
							Name:         "test-3",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-3"}}},
						},
					},
				},
				storage: NewInMemoryStorage(),
			},
			finalStatus: SucceedStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRunner(tt.fields.flow, tt.fields.storage)
			if err := r.Start(context.TODO()); err != nil {
				t.Errorf("Start() error = %v", err)
			}
			if err := waitingFlowFinish(t, tt.fields.flow.ID, tt.finalStatus, tt.fields.storage); err != nil {
				t.Errorf("waitingFlowFinish() error = %v", err)
			}
		})
	}
}

func Test_runner_ReStart(t *testing.T) {
	type fields struct {
		flow    *Flow
		storage Storage
	}
	tests := []struct {
		name        string
		fields      fields
		finalStatus string
	}{
		{
			name: "test-simple-flow-1",
			fields: fields{
				flow: &Flow{
					ID:            "test-simple-flow-1",
					Executor:      "test",
					Status:        PausedStatus,
					ControlPolicy: ControlPolicy{FailedPolicy: PolicyFastFailed},
					Tasks: []Task{
						{
							Name:         "test-1",
							Status:       SucceedStatus,
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-1"}}},
							Next:         NextTask{OnSucceed: "test-2"},
						},
						{
							Name:         "test-2",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-2"}}},
						},
					},
				},
				storage: NewInMemoryStorage(),
			},
			finalStatus: SucceedStatus,
		},
		{
			name: "test-simple-flow-2",
			fields: fields{
				flow: &Flow{
					ID:            "test-simple-flow-2",
					Executor:      "test",
					Status:        RunningStatus,
					ControlPolicy: ControlPolicy{FailedPolicy: PolicyFastFailed},
					Tasks: []Task{
						{
							Name:         "test-1",
							Status:       SucceedStatus,
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-1"}}},
							Next:         NextTask{OnSucceed: "test-2"},
						},
						{
							Name:         "test-2",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-2"}}},
						},
					},
				},
				storage: NewInMemoryStorage(),
			},
			finalStatus: SucceedStatus,
		},
		{
			name: "test-simple-flow-3",
			fields: fields{
				flow: &Flow{
					ID:            "test-simple-flow-3",
					Executor:      "test",
					Status:        RunningStatus,
					ControlPolicy: ControlPolicy{FailedPolicy: PolicyFastFailed},
					Tasks: []Task{
						{
							Name:         "test-1",
							Status:       FailedStatus,
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "failed"}}},
							Next:         NextTask{OnFailed: "test-2"},
						},
						{
							Name:         "test-2",
							OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-2"}}},
						},
					},
				},
				storage: NewInMemoryStorage(),
			},
			finalStatus: SucceedStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRunner(tt.fields.flow, tt.fields.storage)
			go func() {
				if tt.fields.flow.Status == PausedStatus {
					time.Sleep(time.Second * 5)
					if err := r.Resume(); err != nil {
						t.Errorf("Resume() error = %v", err)
					}
				}
			}()
			if err := r.Start(context.TODO()); err != nil {
				t.Errorf("Start() error = %v", err)
			}
			if err := waitingFlowFinish(t, tt.fields.flow.ID, tt.finalStatus, tt.fields.storage); err != nil {
				t.Errorf("waitingFlowFinish() error = %v", err)
			}
		})
	}
}

func waitingFlowStatus(t *testing.T, fID, status string, s Storage) error {
	for i := 0; i < 100; i++ {
		f, err := s.GetFlow(context.TODO(), fID)
		if err != nil {
			return err
		}
		t.Logf("flow %s want %s, current status: %s", fID, status, f.Status)
		if f.Status == status {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout")
}

func waitingFlowFinish(t *testing.T, fID, status string, s Storage) error {
	for i := 0; i < 100; i++ {
		f, err := s.GetFlow(context.TODO(), fID)
		if err != nil {
			return err
		}
		t.Logf("flow %s status: %s", fID, f.Status)
		if IsFinishedStatus(f.Status) {
			if f.Status != status {
				return fmt.Errorf("want %s, got %s", status, f.Status)
			}
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout")
}
