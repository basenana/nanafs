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
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/workflow"
)

// ListWorkflows retrieves available workflows
func (s *ServicesV1) ListWorkflows(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowList, err := s.workflow.ListWorkflows(ctx.Request.Context(), caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	workflows := make([]*WorkflowInfo, 0, len(workflowList))
	for _, w := range workflowList {
		workflows = append(workflows, toWorkflowInfo(w))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListWorkflowsResponse{Workflows: workflows})
}

// ListWorkflowJobs retrieves jobs for a specific workflow
func (s *ServicesV1) ListWorkflowJobs(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	jobs, err := s.workflow.ListJobs(ctx.Request.Context(), caller.Namespace, workflowID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	result := make([]*WorkflowJobDetail, 0, len(jobs))
	for _, j := range jobs {
		result = append(result, toWorkflowJobDetail(j))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListWorkflowJobsResponse{Jobs: result})
}

// TriggerWorkflow triggers a workflow
func (s *ServicesV1) TriggerWorkflow(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req TriggerWorkflowRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	_, err := s.workflow.GetWorkflow(ctx.Request.Context(), caller.Namespace, workflowID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	timeout := time.Second * 60 * 10
	if req.Timeout > 0 {
		timeout = time.Second * time.Duration(req.Timeout)
	}

	job, err := s.workflow.TriggerWorkflow(ctx.Request.Context(), caller.Namespace, workflowID,
		types.WorkflowTarget{Entries: []string{req.URI}},
		workflow.JobAttr{Reason: req.Reason, Timeout: timeout},
	)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &TriggerWorkflowResponse{JobID: job.Id})
}

// CreateWorkflow creates a new workflow
func (s *ServicesV1) CreateWorkflow(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req CreateWorkflowRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	workflow := &types.Workflow{
		Name:      req.Name,
		Trigger:   req.Trigger,
		Nodes:     req.Nodes,
		Enable:    req.Enable,
		QueueName: req.QueueName,
	}

	result, err := s.workflow.CreateWorkflow(ctx.Request.Context(), caller.Namespace, workflow)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusCreated, gin.H{"workflow": toWorkflowInfo(result)})
}

// GetWorkflow retrieves a specific workflow
func (s *ServicesV1) GetWorkflow(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	workflow, err := s.workflow.GetWorkflow(ctx.Request.Context(), caller.Namespace, workflowID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"workflow": toWorkflowInfo(workflow)})
}

// UpdateWorkflow updates an existing workflow
func (s *ServicesV1) UpdateWorkflow(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	var req UpdateWorkflowRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	existing, err := s.workflow.GetWorkflow(ctx.Request.Context(), caller.Namespace, workflowID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", err)
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Trigger.LocalFileWatch != nil || req.Trigger.RSS != nil || req.Trigger.Interval != nil {
		existing.Trigger = req.Trigger
	}
	if len(req.Nodes) > 0 {
		existing.Nodes = req.Nodes
	}
	if req.Enable != nil {
		existing.Enable = *req.Enable
	}
	if req.QueueName != "" {
		existing.QueueName = req.QueueName
	}

	result, err := s.workflow.UpdateWorkflow(ctx.Request.Context(), caller.Namespace, existing)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"workflow": toWorkflowInfo(result)})
}

// DeleteWorkflow deletes a workflow
func (s *ServicesV1) DeleteWorkflow(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	err := s.workflow.DeleteWorkflow(ctx.Request.Context(), caller.Namespace, workflowID)
	if err != nil {
		if strings.Contains(err.Error(), "workflow not found") {
			apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", err)
		} else {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		}
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"message": "workflow deleted"})
}

// GetJob retrieves a specific job
func (s *ServicesV1) GetJob(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowID := ctx.Param("id")
	jobID := ctx.Param("jobId")
	if workflowID == "" || jobID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id or job id"})
		return
	}

	job, err := s.workflow.GetJob(ctx.Request.Context(), caller.Namespace, workflowID, jobID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"job": toWorkflowJobDetail(job)})
}

// PauseJob pauses a running job
func (s *ServicesV1) PauseJob(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	jobID := ctx.Param("jobId")
	if jobID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	err := s.workflow.PauseWorkflowJob(ctx.Request.Context(), caller.Namespace, jobID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"message": "job paused"})
}

// ResumeJob resumes a paused job
func (s *ServicesV1) ResumeJob(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	jobID := ctx.Param("jobId")
	if jobID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	err := s.workflow.ResumeWorkflowJob(ctx.Request.Context(), caller.Namespace, jobID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"message": "job resumed"})
}

// CancelJob cancels a job
func (s *ServicesV1) CancelJob(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	jobID := ctx.Param("jobId")
	if jobID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	err := s.workflow.CancelWorkflowJob(ctx.Request.Context(), caller.Namespace, jobID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"message": "job cancelled"})
}
