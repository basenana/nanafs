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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/basenana/friday/core/session"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/friday"
	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
)

type ChatRequest struct {
	Message        string   `json:"message"`
	SessionID      string   `json:"session_id,omitempty"`
	ContextEntries []string `json:"context_entries,omitempty"`
}

type CreateSessionRequest struct {
	Name string `json:"name,omitempty"`
}

type CreateSessionResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	CreatedAt string `json:"created_at"`
}

type ListSessionsResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

type SessionInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	MessageCount int    `json:"message_count"`
}

type GetSessionResponse struct {
	Meta     SessionInfo      `json:"meta"`
	Messages []SessionMessage `json:"messages"`
}

type SessionMessage struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	Reasoning string `json:"reasoning,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	Time      string `json:"time"`
}

type RenameSessionRequest struct {
	Name string `json:"name"`
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

	if req.SessionID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("session id is required"))
		return
	}

	if req.Message == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("message is required"))
		return
	}

	fbot, err := s.fridayManager.GetFriday(caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("get friday: %w", err))
		return
	}

	var sess *friday.Session
	sess, err = fbot.OpenSession(ctx.Request.Context(), req.SessionID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("load session: %w", err))
		return
	}

	s.handleSSEStream(ctx, fbot, caller.Namespace, req.SessionID, sess, req.Message)
}

type FridayMessage struct {
	Reasoning string         `json:"reasoning,omitempty"`
	Content   string         `json:"content,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

func (s *ServicesV1) handleSSEStream(ctx *gin.Context, fbot *friday.Friday, namespace, sessionID string, sess *session.Session, userMessage string) {
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")

	flusher, ok := ctx.Writer.(http.Flusher)
	if !ok {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "STREAM_NOT_SUPPORTED", fmt.Errorf("streaming not supported"))
		return
	}

	eventCh, closeF := events.SubscribeFridayEvents(namespace, sessionID)
	defer closeF()

	resp := fbot.Chat(ctx.Request.Context(), sess, userMessage)
	if resp == nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("chat response is nil"))
		return
	}

	store := fbot.GetStore()
	err := store.AppendMessage(context.Background(), sessionID,
		&friday.SessionMessage{
			Type:    "user",
			Content: userMessage,
			Time:    time.Now().Format(time.RFC3339),
		},
	)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("store append user message: %w", err))
		return
	}

	var (
		assistantContent   string
		assistantReasoning string
		assistantEvents    []*friday.Event
	)

	sendMessage := func(msgType string, obj any) {
		data, err := json.Marshal(obj)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(ctx.Writer, "event: %s\ndata: %s\n\n", msgType, data)
		flusher.Flush()
	}

Stream:
	for {
		select {
		case err := <-resp.Error():
			if err != nil {
				sendMessage("MESSAGE-APPEND", FridayMessage{Reasoning: err.Error()})
				break Stream
			}
		case delta, ok := <-resp.Deltas():
			if !ok {
				break Stream
			}
			assistantReasoning += delta.Reasoning
			assistantContent += delta.Content
			sendMessage("MESSAGE-APPEND", FridayMessage{Reasoning: delta.Reasoning, Content: delta.Content})
		case evtRaw, ok := <-eventCh:
			if !ok {
				break Stream
			}
			evt, ok := evtRaw.(*friday.Event)
			if !ok {
				s.logger.Warnw("unexpected event type", "type", fmt.Sprintf("%T", evtRaw))
				continue
			}
			assistantEvents = append(assistantEvents, evt)
			sendMessage("EVENT-UPDATE", evt)
		}
	}

	go func() {
		var (
			now           = time.Now().Format(time.RFC3339)
			agentMessages []*friday.SessionMessage
		)

		for _, evt := range assistantEvents {
			agentMessages = append(agentMessages, &friday.SessionMessage{
				Type:     "event",
				Event:    evt.Event,
				EntryURI: evt.EntryURI,
				Time:     evt.Time.Format(time.RFC3339),
			})
		}

		if len(assistantReasoning) > 0 || len(assistantContent) > 0 {
			agentMessages = append(agentMessages, &friday.SessionMessage{
				Type:      "assistant",
				Content:   assistantContent,
				Reasoning: assistantReasoning,
				Time:      now,
			})
		}

		if len(agentMessages) > 0 {
			_ = store.AppendMessage(context.Background(), sessionID, agentMessages...)
		}
	}()

	_, _ = fmt.Fprintf(ctx.Writer, "event: DONE\ndata: {}\n\n")
}

func (s *ServicesV1) CreateSession(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	if s.fridayManager == nil {
		apitool.ErrorResponse(ctx, http.StatusServiceUnavailable, "LLM_NOT_ENABLED", fmt.Errorf("LLM service is not enabled"))
		return
	}

	var req CreateSessionRequest
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("read request body: %w", err))
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("parse request: %w", err))
		return
	}

	if req.Name == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("name is required"))
		return
	}

	fbot, err := s.fridayManager.GetFriday(caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("get friday: %w", err))
		return
	}

	store := fbot.GetStore()
	meta, err := store.CreateSession(ctx.Request.Context(), req.Name)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("create session: %w", err))
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, CreateSessionResponse{
		ID:        meta.ID,
		Name:      meta.Name,
		CreatedAt: meta.CreatedAt.Format(time.RFC3339),
	})
}

func (s *ServicesV1) ListSessions(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	if s.fridayManager == nil {
		apitool.ErrorResponse(ctx, http.StatusServiceUnavailable, "LLM_NOT_ENABLED", fmt.Errorf("LLM service is not enabled"))
		return
	}

	fbot, err := s.fridayManager.GetFriday(caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("get friday: %w", err))
		return
	}

	store := fbot.GetStore()
	sessions, err := store.ListSessions(ctx.Request.Context())
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("list sessions: %w", err))
		return
	}

	sessionInfos := make([]SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		sessionInfos = append(sessionInfos, SessionInfo{
			ID:        sess.ID,
			Name:      sess.Name,
			CreatedAt: sess.CreatedAt.Format(time.RFC3339),
			UpdatedAt: sess.UpdatedAt.Format(time.RFC3339),
		})
	}

	apitool.JsonResponse(ctx, http.StatusOK, ListSessionsResponse{
		Sessions: sessionInfos,
	})
}

func (s *ServicesV1) GetSession(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	if s.fridayManager == nil {
		apitool.ErrorResponse(ctx, http.StatusServiceUnavailable, "LLM_NOT_ENABLED", fmt.Errorf("LLM service is not enabled"))
		return
	}

	sessionID := ctx.Param("id")
	if sessionID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("session id is required"))
		return
	}

	fbot, err := s.fridayManager.GetFriday(caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("get friday: %w", err))
		return
	}

	store := fbot.GetStore()
	meta, err := store.GetSession(ctx.Request.Context(), sessionID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", fmt.Errorf("session not found: %w", err))
		return
	}

	messages, err := store.GetMessages(ctx.Request.Context(), sessionID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("get messages: %w", err))
		return
	}

	sessionMessages := make([]SessionMessage, 0, len(messages))
	for _, msg := range messages {
		sessionMessages = append(sessionMessages, SessionMessage{
			Type:      msg.Type,
			Content:   msg.Content,
			Reasoning: msg.Reasoning,
			ToolName:  msg.ToolName,
			Time:      msg.Time,
		})
	}

	apitool.JsonResponse(ctx, http.StatusOK, GetSessionResponse{
		Meta: SessionInfo{
			ID:        meta.ID,
			Name:      meta.Name,
			CreatedAt: meta.CreatedAt.Format(time.RFC3339),
			UpdatedAt: meta.UpdatedAt.Format(time.RFC3339),
		},
		Messages: sessionMessages,
	})
}

func (s *ServicesV1) DeleteSession(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	if s.fridayManager == nil {
		apitool.ErrorResponse(ctx, http.StatusServiceUnavailable, "LLM_NOT_ENABLED", fmt.Errorf("LLM service is not enabled"))
		return
	}

	sessionID := ctx.Param("id")
	if sessionID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("session id is required"))
		return
	}

	fbot, err := s.fridayManager.GetFriday(caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("get friday: %w", err))
		return
	}

	store := fbot.GetStore()
	err = store.DeleteSession(ctx.Request.Context(), sessionID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("delete session: %w", err))
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"message": "deleted"})
}

func (s *ServicesV1) RenameSession(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	if s.fridayManager == nil {
		apitool.ErrorResponse(ctx, http.StatusServiceUnavailable, "LLM_NOT_ENABLED", fmt.Errorf("LLM service is not enabled"))
		return
	}

	sessionID := ctx.Param("id")
	if sessionID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("session id is required"))
		return
	}

	var req RenameSessionRequest
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("read request body: %w", err))
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("parse request: %w", err))
		return
	}

	if req.Name == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("name is required"))
		return
	}

	fbot, err := s.fridayManager.GetFriday(caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("get friday: %w", err))
		return
	}

	store := fbot.GetStore()
	err = store.RenameSession(ctx.Request.Context(), sessionID, req.Name)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Errorf("rename session: %w", err))
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"message": "renamed"})
}
