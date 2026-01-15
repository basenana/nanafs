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
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/workflow"
)

// @Summary List workflows
// @Description Retrieve all available workflows
// @Tags Workflows
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param sort query string false "Sort field (name, created_at, updated_at)"
// @Param order query string false "Sort order (asc, desc)"
// @Success 200 {object} ListWorkflowsResponse
// @Router /api/v1/workflows [get]
func (s *ServicesV1) ListWorkflows(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req PaginationRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	pg := types.NewPaginationWithSort(req.Page, req.PageSize, req.Sort, req.Order)
	pagedCtx := types.WithPagination(ctx.Request.Context(), pg)

	workflowList, err := s.workflow.ListWorkflows(pagedCtx, caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	workflows := make([]*WorkflowInfo, 0, len(workflowList))
	for _, w := range workflowList {
		workflows = append(workflows, toWorkflowInfo(w))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListWorkflowsResponse{
		Workflows:  workflows,
		Pagination: &PaginationInfo{Page: pg.Page, PageSize: pg.PageSize},
	})
}

// @Summary List workflow jobs
// @Description Retrieve all jobs for a specific workflow
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param sort query string false "Sort field (created_at, updated_at)"
// @Param order query string false "Sort order (asc, desc)"
// @Success 200 {object} ListWorkflowJobsResponse
// @Router /api/v1/workflows/{id}/jobs [get]
func (s *ServicesV1) ListWorkflowJobs(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("invalid workflow id"))
		return
	}

	var req PaginationRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	pg := types.NewPaginationWithSort(req.Page, req.PageSize, req.Sort, req.Order)
	pagedCtx := types.WithPagination(ctx.Request.Context(), pg)

	jobs, err := s.workflow.ListJobs(pagedCtx, caller.Namespace, workflowID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	result := make([]*WorkflowJobDetail, 0, len(jobs))
	for _, j := range jobs {
		result = append(result, toWorkflowJobDetail(j))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListWorkflowJobsResponse{
		Jobs:       result,
		Pagination: &PaginationInfo{Page: pg.Page, PageSize: pg.PageSize},
	})
}

// @Summary Trigger workflow
// @Description Manually trigger a workflow to run
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param request body TriggerWorkflowRequest true "Trigger workflow request"
// @Success 200 {object} TriggerWorkflowResponse
// @Router /api/v1/workflows/{id}/trigger [post]
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
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("invalid workflow id"))
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

// @Summary Create workflow
// @Description Create a new workflow
// @Tags Workflows
// @Accept json
// @Produce json
// @Param request body CreateWorkflowRequest true "Create workflow request"
// @Success 201 {object} CreateWorkflowResponse
// @Router /api/v1/workflows [post]
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

	apitool.JsonResponse(ctx, http.StatusCreated, &CreateWorkflowResponse{
		Workflow: toWorkflowInfo(result),
	})
}

// @Summary Get workflow
// @Description Retrieve a specific workflow by ID
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Success 200 {object} WorkflowResponse
// @Router /api/v1/workflows/{id} [get]
func (s *ServicesV1) GetWorkflow(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("invalid workflow id"))
		return
	}

	workflow, err := s.workflow.GetWorkflow(ctx.Request.Context(), caller.Namespace, workflowID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &WorkflowResponse{
		Workflow: toWorkflowDetail(workflow),
	})
}

// @Summary Update workflow
// @Description Update an existing workflow
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param request body UpdateWorkflowRequest true "Update workflow request"
// @Success 200 {object} WorkflowResponse
// @Router /api/v1/workflows/{id} [put]
func (s *ServicesV1) UpdateWorkflow(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("invalid workflow id"))
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

	apitool.JsonResponse(ctx, http.StatusOK, &WorkflowResponse{
		Workflow: toWorkflowDetail(result),
	})
}

// @Summary Delete workflow
// @Description Delete a workflow by ID
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Success 200 {object} MessageResponse
// @Router /api/v1/workflows/{id} [delete]
func (s *ServicesV1) DeleteWorkflow(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("invalid workflow id"))
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

	apitool.JsonResponse(ctx, http.StatusOK, &MessageResponse{Message: "workflow deleted"})
}

// @Summary Get workflow job
// @Description Retrieve a specific job by workflow ID and job ID
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param jobId path string true "Job ID"
// @Success 200 {object} WorkflowJobResponse
// @Router /api/v1/workflows/{id}/jobs/{jobId} [get]
func (s *ServicesV1) GetJob(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	workflowID := ctx.Param("id")
	jobID := ctx.Param("jobId")
	if workflowID == "" || jobID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("invalid workflow id or job id"))
		return
	}

	job, err := s.workflow.GetJob(ctx.Request.Context(), caller.Namespace, workflowID, jobID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &WorkflowJobResponse{
		Job: toWorkflowJobDetail(job),
	})
}

// @Summary Pause job
// @Description Pause a running workflow job
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param jobId path string true "Job ID"
// @Success 200 {object} MessageResponse
// @Router /api/v1/workflows/{id}/jobs/{jobId}/pause [post]
func (s *ServicesV1) PauseJob(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	jobID := ctx.Param("jobId")
	if jobID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("invalid job id"))
		return
	}

	err := s.workflow.PauseWorkflowJob(ctx.Request.Context(), caller.Namespace, jobID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &MessageResponse{Message: "job paused"})
}

// @Summary Resume job
// @Description Resume a paused workflow job
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param jobId path string true "Job ID"
// @Success 200 {object} MessageResponse
// @Router /api/v1/workflows/{id}/jobs/{jobId}/resume [post]
func (s *ServicesV1) ResumeJob(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	jobID := ctx.Param("jobId")
	if jobID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("invalid job id"))
		return
	}

	err := s.workflow.ResumeWorkflowJob(ctx.Request.Context(), caller.Namespace, jobID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &MessageResponse{Message: "job resumed"})
}

// @Summary Cancel job
// @Description Cancel a workflow job
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param jobId path string true "Job ID"
// @Success 200 {object} MessageResponse
// @Router /api/v1/workflows/{id}/jobs/{jobId}/cancel [post]
func (s *ServicesV1) CancelJob(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	jobID := ctx.Param("jobId")
	if jobID == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("invalid job id"))
		return
	}

	err := s.workflow.CancelWorkflowJob(ctx.Request.Context(), caller.Namespace, jobID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &MessageResponse{Message: "job cancelled"})
}
