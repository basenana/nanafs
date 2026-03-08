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

package friday

import (
	"testing"

	fridaytypes "github.com/basenana/friday/core/types"
)

func TestMessageToSessionMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    *fridaytypes.Message
		expected *SessionMessage
	}{
		{
			name: "user message",
			input: &fridaytypes.Message{
				UserMessage: "Hello",
				Time:        "2026-02-19T10:00:00Z",
			},
			expected: &SessionMessage{
				Type:    "user",
				Content: "Hello",
				Time:    "2026-02-19T10:00:00Z",
			},
		},
		{
			name: "assistant message",
			input: &fridaytypes.Message{
				AssistantMessage:   "Hi there",
				AssistantReasoning: "Thinking...",
				Time:               "2026-02-19T10:00:01Z",
			},
			expected: &SessionMessage{
				Type:      "assistant",
				Content:   "Hi there",
				Reasoning: "Thinking...",
				Time:      "2026-02-19T10:00:01Z",
			},
		},
		{
			name: "agent message fallback",
			input: &fridaytypes.Message{
				AgentMessage:       "Hi there",
				AssistantReasoning: "Thinking...",
				Time:               "2026-02-19T10:00:02Z",
			},
			expected: &SessionMessage{
				Type:      "assistant",
				Content:   "Hi there",
				Reasoning: "Thinking...",
				Time:      "2026-02-19T10:00:02Z",
			},
		},
		{
			name: "tool message",
			input: &fridaytypes.Message{
				ToolName:      "file_list",
				ToolArguments: "{}",
				ToolContent:   "file1.txt\nfile2.txt",
				ToolCallID:    "call_123",
				Time:          "2026-02-19T10:00:03Z",
			},
			expected: &SessionMessage{
				Type:          "tool",
				ToolName:      "file_list",
				ToolArguments: "{}",
				ToolContent:   "file1.txt\nfile2.txt",
				ToolCallID:    "call_123",
				Time:          "2026-02-19T10:00:03Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MessageToSessionMessage(tt.input)
			if result.Type != tt.expected.Type {
				t.Errorf("Type: got %s, want %s", result.Type, tt.expected.Type)
			}
			if result.Content != tt.expected.Content {
				t.Errorf("Content: got %s, want %s", result.Content, tt.expected.Content)
			}
			if result.Reasoning != tt.expected.Reasoning {
				t.Errorf("Reasoning: got %s, want %s", result.Reasoning, tt.expected.Reasoning)
			}
			if result.ToolName != tt.expected.ToolName {
				t.Errorf("ToolName: got %s, want %s", result.ToolName, tt.expected.ToolName)
			}
			if result.Time != tt.expected.Time {
				t.Errorf("Time: got %s, want %s", result.Time, tt.expected.Time)
			}
		})
	}
}

func TestSessionMessageToMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    *SessionMessage
		expected *fridaytypes.Message
	}{
		{
			name: "user message",
			input: &SessionMessage{
				Type:    "user",
				Content: "Hello",
				Time:    "2026-02-19T10:00:00Z",
			},
			expected: &fridaytypes.Message{
				UserMessage: "Hello",
				Time:        "2026-02-19T10:00:00Z",
			},
		},
		{
			name: "assistant message",
			input: &SessionMessage{
				Type:      "assistant",
				Content:   "Hi there",
				Reasoning: "Thinking...",
				Time:      "2026-02-19T10:00:01Z",
			},
			expected: &fridaytypes.Message{
				AssistantMessage:   "Hi there",
				AssistantReasoning: "Thinking...",
				Time:               "2026-02-19T10:00:01Z",
			},
		},
		{
			name: "tool message",
			input: &SessionMessage{
				Type:          "tool",
				ToolName:      "file_list",
				ToolArguments: "{}",
				ToolContent:   "file1.txt",
				ToolCallID:    "call_123",
				Time:          "2026-02-19T10:00:02Z",
			},
			expected: &fridaytypes.Message{
				ToolName:      "file_list",
				ToolArguments: "{}",
				ToolContent:   "file1.txt",
				ToolCallID:    "call_123",
				Time:          "2026-02-19T10:00:02Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SessionMessageToMessage(tt.input)
			if result.UserMessage != tt.expected.UserMessage {
				t.Errorf("UserMessage: got %s, want %s", result.UserMessage, tt.expected.UserMessage)
			}
			if result.AssistantMessage != tt.expected.AssistantMessage {
				t.Errorf("AssistantMessage: got %s, want %s", result.AssistantMessage, tt.expected.AssistantMessage)
			}
			if result.AssistantReasoning != tt.expected.AssistantReasoning {
				t.Errorf("AssistantReasoning: got %s, want %s", result.AssistantReasoning, tt.expected.AssistantReasoning)
			}
			if result.ToolName != tt.expected.ToolName {
				t.Errorf("ToolName: got %s, want %s", result.ToolName, tt.expected.ToolName)
			}
		})
	}
}
