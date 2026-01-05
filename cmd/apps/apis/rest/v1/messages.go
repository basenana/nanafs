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
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
	"github.com/basenana/nanafs/pkg/types"
)

// ListMessages retrieves notifications/messages
func (s *ServicesV1) ListMessages(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	all := ctx.Query("all") == "true"

	notifications, err := s.notify.ListNotifications(ctx.Request.Context(), caller.Namespace)
	if err != nil {
		// Return empty list for memory metastore that doesn't support notifications
		notifications = nil
	}

	result := make([]*Message, 0, len(notifications))
	for _, m := range notifications {
		if !all && m.Status == types.NotificationRead {
			continue
		}
		result = append(result, &Message{
			ID:      m.ID,
			Title:   m.Title,
			Message: m.Message,
			Type:    string(m.Type),
			Source:  m.Source,
			Action:  "",
			Status:  string(m.Status),
			Time:    m.Time,
		})
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListMessagesResponse{Messages: result})
}

// ReadMessages marks messages as read
func (s *ServicesV1) ReadMessages(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req ReadMessagesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	for _, id := range req.MessageIDList {
		if err := s.notify.MarkRead(ctx.Request.Context(), caller.Namespace, id); err != nil {
			// Ignore errors for memory metastore that doesn't support notifications
			continue
		}
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ReadMessagesResponse{Success: true})
}
