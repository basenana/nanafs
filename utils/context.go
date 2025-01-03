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

package utils

import (
	"context"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"net/http"
)

const (
	apiTraceContextKey = "api.trace"
	jobTraceContextKey = "workflow.trace"
)

func NewApiContext(r *http.Request) context.Context {
	return context.WithValue(r.Context(), apiTraceContextKey, uuid.New().String())
}

func NewWorkflowJobContext(ctx context.Context, jobID string) context.Context {
	return context.WithValue(ctx, jobTraceContextKey, jobID)
}

func WorkflowJobLogger(ctx context.Context, log *zap.SugaredLogger) *zap.SugaredLogger {
	return contextLog(ctx, "job", jobTraceContextKey, log)
}

func contextLog(ctx context.Context, objectType, traceKey string, log *zap.SugaredLogger) *zap.SugaredLogger {
	traceID := ""
	traceObj := ctx.Value(traceKey)
	if traceObj != nil {
		traceID = traceObj.(string)
	}
	return log.With(zap.String(objectType, traceID))
}
