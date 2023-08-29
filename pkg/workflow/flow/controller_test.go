package flow

import (
	"context"
	"testing"
)

func TestController_PauseFlow(t *testing.T) {
	tests := []struct {
		name    string
		flow    *Flow
		storage Storage
	}{
		{
			name: "test-simple-flow-1",
			flow: &Flow{
				ID:       "test-simple-flow-1",
				Executor: "test",
				Tasks: []Task{
					{
						Name:         "test-1",
						OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"sleep", "5"}}},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.storage.SaveFlow(context.Background(), tt.flow); err != nil {
				t.Errorf("SaveFlow() error = %v", err)
			}

			c := NewFlowController(tt.storage)
			if err := c.TriggerFlow(context.Background(), tt.flow.ID); err != nil {
				t.Errorf("PauseFlow() error = %v", err)
			}

			if err := waitingFlowStatus(t, tt.flow.ID, RunningStatus, tt.storage); err != nil {
				t.Errorf("waitingFlowStatus() wait running error = %v", err)
			}

			if err := c.PauseFlow(tt.flow.ID); err != nil {
				t.Errorf("PauseFlow() error = %v", err)
			}

			if err := waitingFlowStatus(t, tt.flow.ID, PausedStatus, tt.storage); err != nil {
				t.Errorf("waitingFlowStatus() wait paused error = %v", err)
			}

			if err := c.ResumeFlow(tt.flow.ID); err != nil {
				t.Errorf("ResumeFlow() error = %v", err)
			}

			if err := waitingFlowFinish(t, tt.flow.ID, SucceedStatus, tt.storage); err != nil {
				t.Errorf("waitingFlowFinish() wait succeed error = %v", err)
			}
		})
	}
}

func TestController_CancelFlow(t *testing.T) {
	tests := []struct {
		name       string
		flow       *Flow
		storage    Storage
		needPaused bool
	}{
		{
			name: "",
			flow: &Flow{
				ID:       "test-simple-flow-1",
				Executor: "test",
				Tasks: []Task{
					{
						Name:         "test-1",
						OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"sleep", "5"}}},
						Next:         NextTask{OnSucceed: "test-2"},
					},
					{
						Name:         "test-2",
						OperatorSpec: Spec{Type: "shell", Script: &Script{Command: []string{"echo", "test-2"}}},
					},
				},
			},
			storage:    NewInMemoryStorage(),
			needPaused: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.storage.SaveFlow(context.Background(), tt.flow); err != nil {
				t.Errorf("SaveFlow() error = %v", err)
			}

			c := NewFlowController(tt.storage)
			if err := c.TriggerFlow(context.Background(), tt.flow.ID); err != nil {
				t.Errorf("PauseFlow() error = %v", err)
			}

			if err := waitingFlowStatus(t, tt.flow.ID, RunningStatus, tt.storage); err != nil {
				t.Errorf("waitingFlowStatus() wait running error = %v", err)
			}

			if tt.needPaused {
				if err := c.PauseFlow(tt.flow.ID); err != nil {
					t.Errorf("PauseFlow() error = %v", err)
				}

				if err := waitingFlowStatus(t, tt.flow.ID, PausedStatus, tt.storage); err != nil {
					t.Errorf("waitingFlowStatus() wait paused error = %v", err)
				}
			}

			if err := c.CancelFlow(tt.flow.ID); err != nil {
				t.Errorf("CancelFlow() error = %v", err)
			}
			if err := waitingFlowFinish(t, tt.flow.ID, CanceledStatus, tt.storage); err != nil {
				t.Errorf("waitingFlowFinish() wait succeed error = %v", err)
			}
		})
	}
}
