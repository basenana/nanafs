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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/gin-gonic/gin"
)

var _ = Describe("REST V1 Workflow API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	Describe("CreateWorkflow", func() {
		It("should create a new workflow", func() {
			reqBody := CreateWorkflowRequest{
				Name:      "test-workflow",
				Enable:    true,
				QueueName: "default",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))
		})

		It("should return error for missing name", func() {
			reqBody := CreateWorkflowRequest{
				Enable: true,
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GetWorkflow", func() {
		BeforeEach(func() {
			reqBody := CreateWorkflowRequest{
				Name:      "test-workflow-for-get",
				Enable:    true,
				QueueName: "default",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusCreated))
		})

		It("should get a workflow by id", func() {
			req, err := http.NewRequest("GET", "/api/v1/workflows/wf-1", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 404 for non-existent workflow", func() {
			req, err := http.NewRequest("GET", "/api/v1/workflows/non-existent", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("UpdateWorkflow", func() {
		BeforeEach(func() {
			reqBody := CreateWorkflowRequest{
				Name:      "workflow-to-update",
				Enable:    false,
				QueueName: "default",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusCreated))
		})

		It("should update a workflow", func() {
			reqBody := UpdateWorkflowRequest{
				Name:   "updated-name",
				Enable: BoolPtr(true),
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/workflows/wf-1", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 404 for non-existent workflow", func() {
			reqBody := UpdateWorkflowRequest{
				Name: "updated-name",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/workflows/non-existent", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("DeleteWorkflow", func() {
		BeforeEach(func() {
			reqBody := CreateWorkflowRequest{
				Name:      "workflow-to-delete",
				Enable:    true,
				QueueName: "default",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusCreated))
		})

		It("should delete a workflow", func() {
			req, err := http.NewRequest("DELETE", "/api/v1/workflows/wf-1", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 404 for non-existent workflow", func() {
			req, err := http.NewRequest("DELETE", "/api/v1/workflows/non-existent", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Workflow CRUD Full Flow", func() {
		It("should create, get, update, and delete workflow", func() {
			createReq := CreateWorkflowRequest{
				Name:      "full-flow-workflow",
				Enable:    true,
				QueueName: "default",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
			Expect(createW.Code).To(Equal(http.StatusCreated))

			getReq, _ := http.NewRequest("GET", "/api/v1/workflows/wf-1", nil)
			getReq.Header.Set("X-Namespace", types.DefaultNamespace)
			getW := httptest.NewRecorder()
			router.ServeHTTP(getW, getReq)
			Expect(getW.Code).To(Equal(http.StatusOK))

			updateReq := UpdateWorkflowRequest{
				Name:   "updated-full-flow-workflow",
				Enable: BoolPtr(false),
			}
			updateJson, _ := json.Marshal(updateReq)
			updateHttpReq, _ := http.NewRequest("PUT", "/api/v1/workflows/wf-1", bytes.NewBuffer(updateJson))
			updateHttpReq.Header.Set("Content-Type", "application/json")
			updateHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			updateW := httptest.NewRecorder()
			router.ServeHTTP(updateW, updateHttpReq)
			Expect(updateW.Code).To(Equal(http.StatusOK))

			delReq, _ := http.NewRequest("DELETE", "/api/v1/workflows/wf-1", nil)
			delReq.Header.Set("X-Namespace", types.DefaultNamespace)
			delW := httptest.NewRecorder()
			router.ServeHTTP(delW, delReq)
			Expect(delW.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GetJob", func() {
		BeforeEach(func() {
			createReq := CreateWorkflowRequest{
				Name:      "workflow-for-job",
				Enable:    true,
				QueueName: "default",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)

			triggerReq := TriggerWorkflowRequest{
				URI:    "/test-entry",
				Reason: "test",
			}
			triggerJson, _ := json.Marshal(triggerReq)
			triggerHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/trigger", bytes.NewBuffer(triggerJson))
			triggerHttpReq.Header.Set("Content-Type", "application/json")
			triggerHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			triggerW := httptest.NewRecorder()
			router.ServeHTTP(triggerW, triggerHttpReq)
			Expect(triggerW.Code).To(Equal(http.StatusOK))
		})

		It("should get a job by id", func() {
			req, err := http.NewRequest("GET", "/api/v1/workflows/wf-1/jobs/job-1", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 404 for non-existent job", func() {
			req, err := http.NewRequest("GET", "/api/v1/workflows/wf-1/jobs/non-existent", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("PauseJob", func() {
		BeforeEach(func() {
			createReq := CreateWorkflowRequest{
				Name:      "workflow-for-pause",
				Enable:    true,
				QueueName: "default",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)

			triggerReq := TriggerWorkflowRequest{
				URI:    "/test-entry",
				Reason: "test",
			}
			triggerJson, _ := json.Marshal(triggerReq)
			triggerHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/trigger", bytes.NewBuffer(triggerJson))
			triggerHttpReq.Header.Set("Content-Type", "application/json")
			triggerHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			triggerW := httptest.NewRecorder()
			router.ServeHTTP(triggerW, triggerHttpReq)
		})

		It("should pause a running job", func() {
			req, err := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/job-1/pause", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return error for non-existent job", func() {
			req, err := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/non-existent/pause", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("ResumeJob", func() {
		BeforeEach(func() {
			createReq := CreateWorkflowRequest{
				Name:      "workflow-for-resume",
				Enable:    true,
				QueueName: "default",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)

			triggerReq := TriggerWorkflowRequest{
				URI:    "/test-entry",
				Reason: "test",
			}
			triggerJson, _ := json.Marshal(triggerReq)
			triggerHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/trigger", bytes.NewBuffer(triggerJson))
			triggerHttpReq.Header.Set("Content-Type", "application/json")
			triggerHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			triggerW := httptest.NewRecorder()
			router.ServeHTTP(triggerW, triggerHttpReq)

			pauseReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/job-1/pause", nil)
			pauseReq.Header.Set("X-Namespace", types.DefaultNamespace)
			pauseW := httptest.NewRecorder()
			router.ServeHTTP(pauseW, pauseReq)
		})

		It("should resume a paused job", func() {
			req, err := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/job-1/resume", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return error for non-existent job", func() {
			req, err := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/non-existent/resume", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("CancelJob", func() {
		BeforeEach(func() {
			createReq := CreateWorkflowRequest{
				Name:      "workflow-for-cancel",
				Enable:    true,
				QueueName: "default",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)

			triggerReq := TriggerWorkflowRequest{
				URI:    "/test-entry",
				Reason: "test",
			}
			triggerJson, _ := json.Marshal(triggerReq)
			triggerHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/trigger", bytes.NewBuffer(triggerJson))
			triggerHttpReq.Header.Set("Content-Type", "application/json")
			triggerHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			triggerW := httptest.NewRecorder()
			router.ServeHTTP(triggerW, triggerHttpReq)
		})

		It("should cancel a running job", func() {
			req, err := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/job-1/cancel", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return error for non-existent job", func() {
			req, err := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/non-existent/cancel", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("Job Operations Full Flow", func() {
		BeforeEach(func() {
			createReq := CreateWorkflowRequest{
				Name:      "workflow-full-job-flow",
				Enable:    true,
				QueueName: "default",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
		})

		It("should trigger, get, pause, resume, and cancel job", func() {
			triggerReq := TriggerWorkflowRequest{
				URI:    "/test-entry",
				Reason: "full flow test",
			}
			triggerJson, _ := json.Marshal(triggerReq)
			triggerHttpReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/trigger", bytes.NewBuffer(triggerJson))
			triggerHttpReq.Header.Set("Content-Type", "application/json")
			triggerHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			triggerW := httptest.NewRecorder()
			router.ServeHTTP(triggerW, triggerHttpReq)
			Expect(triggerW.Code).To(Equal(http.StatusOK))

			getJobReq, _ := http.NewRequest("GET", "/api/v1/workflows/wf-1/jobs/job-1", nil)
			getJobReq.Header.Set("X-Namespace", types.DefaultNamespace)
			getJobW := httptest.NewRecorder()
			router.ServeHTTP(getJobW, getJobReq)
			Expect(getJobW.Code).To(Equal(http.StatusOK))

			pauseReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/job-1/pause", nil)
			pauseReq.Header.Set("X-Namespace", types.DefaultNamespace)
			pauseW := httptest.NewRecorder()
			router.ServeHTTP(pauseW, pauseReq)
			Expect(pauseW.Code).To(Equal(http.StatusOK))

			resumeReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/job-1/resume", nil)
			resumeReq.Header.Set("X-Namespace", types.DefaultNamespace)
			resumeW := httptest.NewRecorder()
			router.ServeHTTP(resumeW, resumeReq)
			Expect(resumeW.Code).To(Equal(http.StatusOK))

			cancelReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/job-1/cancel", nil)
			cancelReq.Header.Set("X-Namespace", types.DefaultNamespace)
			cancelW := httptest.NewRecorder()
			router.ServeHTTP(cancelW, cancelReq)
			Expect(cancelW.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("ListWorkflows", func() {
		It("should return empty list when no workflows", func() {
			testRouter.MockWorkflow.workflows = make(map[string]*types.Workflow)

			req, err := http.NewRequest("GET", "/api/v1/workflows", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return list of workflows", func() {
			reqBody := CreateWorkflowRequest{
				Name:      "test-workflow",
				Enable:    true,
				QueueName: "default",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusCreated))

			listReq, err := http.NewRequest("GET", "/api/v1/workflows", nil)
			Expect(err).Should(BeNil())
			listReq.Header.Set("X-Namespace", types.DefaultNamespace)

			listW := httptest.NewRecorder()
			router.ServeHTTP(listW, listReq)

			Expect(listW.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("ListWorkflowJobs", func() {
		BeforeEach(func() {
			reqBody := CreateWorkflowRequest{
				Name:      "workflow-for-list-jobs",
				Enable:    true,
				QueueName: "default",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusCreated))
		})

		It("should list jobs without status filter", func() {
			req, err := http.NewRequest("GET", "/api/v1/workflows/wf-1/jobs", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should list jobs with single status filter", func() {
			req, err := http.NewRequest("GET", "/api/v1/workflows/wf-1/jobs?status=running", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should list jobs with multiple status filter", func() {
			req, err := http.NewRequest("GET", "/api/v1/workflows/wf-1/jobs?status=running,failed", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 400 for invalid status", func() {
			req, err := http.NewRequest("GET", "/api/v1/workflows/wf-1/jobs?status=invalid_status", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 400 for mixed valid and invalid status", func() {
			req, err := http.NewRequest("GET", "/api/v1/workflows/wf-1/jobs?status=running,invalid", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("TriggerWorkflow", func() {
		BeforeEach(func() {
			reqBody := CreateWorkflowRequest{
				Name:      "workflow-for-trigger",
				Enable:    true,
				QueueName: "default",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/workflows", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		})

		It("should trigger a workflow", func() {
			reqBody := TriggerWorkflowRequest{
				URI:    "/test-entry",
				Reason: "manual trigger",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/workflows/wf-1/trigger", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})
})
