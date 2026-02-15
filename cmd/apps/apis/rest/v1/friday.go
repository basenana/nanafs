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

package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/types"
	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
)

type ChatRequest struct {
	Message string `json:"message"`
}

func (s *ServicesV1) Chat(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	if s.fridayManager == nil {
		apitool.ErrorResponse(ctx, http.StatusServiceUnavailable, "LLM_NOT_ENABLED", fmt.Errorf("LLM service is not enabled"))
		return
	}

	var req ChatRequest
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("read request body: %w", err))
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("parse request: %w", err))
		return
	}

	if req.Message == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("message is required"))
		return
	}

	friday, err := s.fridayManager.GetFriday(caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("get friday: %w", err))
		return
	}

	resp := friday.Chat(ctx.Request.Context(), req.Message)
	if resp == nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("chat response is nil"))
		return
	}

	s.handleSSEStream(ctx, resp)
}

type SSEEventType string

const (
	SSEEventMessage    SSEEventType = "message"
	SSEEventReasoning  SSEEventType = "reasoning"
	SSEEventToolCall   SSEEventType = "tool_call"
	SSEEventToolResult SSEEventType = "tool_result"
	SSEEventDone       SSEEventType = "done"
	SSEEventError      SSEEventType = "error"
)

type SSEEvent struct {
	Type  SSEEventType   `json:"type"`
	Data  string         `json:"data"`
	Extra map[string]any `json:"extra,omitempty"`
}

func (s *ServicesV1) handleSSEStream(ctx *gin.Context, resp *api.Response) {
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")

	flusher, ok := ctx.Writer.(http.Flusher)
	if !ok {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "STREAM_NOT_SUPPORTED", fmt.Errorf("streaming not supported"))
		return
	}

	sendEvent := func(event SSEEvent) {
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		if event.Type == "" || event.Type == SSEEventMessage {
			fmt.Fprintf(ctx.Writer, "data: %s\n\n", data)
		} else {
			fmt.Fprintf(ctx.Writer, "event: %s\ndata: %s\n\n", event.Type, data)
		}
		flusher.Flush()
	}

	for {
		select {
		case delta, ok := <-resp.Deltas():
			if !ok {
				sendEvent(SSEEvent{Type: SSEEventDone})
				return
			}
			s.handleDelta(delta, sendEvent)
		case err := <-resp.Error():
			if err != nil {
				sendEvent(SSEEvent{
					Type: SSEEventError,
					Data: err.Error(),
				})
			}
			sendEvent(SSEEvent{Type: SSEEventDone})
			return
		case <-ctx.Request.Context().Done():
			return
		}
	}
}

func (s *ServicesV1) handleDelta(delta types.Delta, sendEvent func(SSEEvent)) {
	if delta.Reasoning != "" {
		sendEvent(SSEEvent{
			Type: SSEEventReasoning,
			Data: delta.Reasoning,
		})
	}
	if delta.Content != "" {
		sendEvent(SSEEvent{
			Type: SSEEventMessage,
			Data: delta.Content,
		})
	}
}
