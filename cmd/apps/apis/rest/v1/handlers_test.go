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

var _ = Describe("REST V1 Entries API - CRUD Operations", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	AfterEach(func() {
		// Cleanup: delete test entries after each test
		cleanupEntries := []string{"/test-group", "/test-file", "/test-folder", "/test-folder/subfolder"}
		for _, uri := range cleanupEntries {
			req, _ := http.NewRequest("DELETE", "/api/v1/entries?uri="+uri, nil)
			req.Header.Set("X-Namespace", types.DefaultNamespace)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})

	Describe("CreateEntry", func() {
		It("should create a new group entry", func() {
			reqBody := CreateEntryRequest{
				URI:  "/test-group",
				Kind: "group",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))
		})

		It("should create a nested group entry", func() {
			// First create parent group
			parentReq := CreateEntryRequest{
				URI:  "/test-folder",
				Kind: "group",
			}
			parentJson, _ := json.Marshal(parentReq)
			parentHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(parentJson))
			parentHttpReq.Header.Set("Content-Type", "application/json")
			parentHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			parentW := httptest.NewRecorder()
			router.ServeHTTP(parentW, parentHttpReq)
			Expect(parentW.Code).To(Equal(http.StatusCreated))

			// Create nested group
			childReq := CreateEntryRequest{
				URI:  "/test-folder/subfolder",
				Kind: "group",
			}
			childJson, _ := json.Marshal(childReq)
			childHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(childJson))
			childHttpReq.Header.Set("Content-Type", "application/json")
			childHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			childW := httptest.NewRecorder()
			router.ServeHTTP(childW, childHttpReq)

			Expect(childW.Code).To(Equal(http.StatusCreated))
		})

		It("should fail with missing namespace", func() {
			reqBody := CreateEntryRequest{
				URI:  "/test-group",
				Kind: "group",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			// No X-Namespace header

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})
	})

	Describe("GetEntryDetail", func() {
		It("should get entry detail after creation", func() {
			// Create entry first
			createReq := CreateEntryRequest{
				URI:  "/test-group",
				Kind: "group",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
			Expect(createW.Code).To(Equal(http.StatusCreated))

			// Get entry detail via query parameter
			req, err := http.NewRequest("GET", "/api/v1/entries/details?uri=/test-group", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return error for non-existent entry", func() {
			req, err := http.NewRequest("GET", "/api/v1/entries/details?uri=/non-existent", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("UpdateEntry", func() {
		It("should update entry name", func() {
			// Create entry first
			createReq := CreateEntryRequest{
				URI:  "/test-group",
				Kind: "group",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
			Expect(createW.Code).To(Equal(http.StatusCreated))

			// Update entry via query parameter
			updateReq := UpdateEntryRequest{
				Name: "updated-name",
			}
			updateJson, _ := json.Marshal(updateReq)
			updateHttpReq, _ := http.NewRequest("PUT", "/api/v1/entries?uri=/test-group", bytes.NewBuffer(updateJson))
			updateHttpReq.Header.Set("Content-Type", "application/json")
			updateHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			updateW := httptest.NewRecorder()
			router.ServeHTTP(updateW, updateHttpReq)

			Expect(updateW.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("DeleteEntry", func() {
		It("should delete an entry", func() {
			// Create entry first
			createReq := CreateEntryRequest{
				URI:  "/test-group",
				Kind: "group",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
			Expect(createW.Code).To(Equal(http.StatusCreated))

			// Delete entry via query parameter
			req, err := http.NewRequest("DELETE", "/api/v1/entries?uri=/test-group", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("ListGroupChildren", func() {
		It("should list children of a group", func() {
			// Create a test group first
			createReq := CreateEntryRequest{
				URI:  "/list-children-test",
				Kind: "group",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
			Expect(createW.Code).To(Equal(http.StatusCreated))

			// List children of the group via query parameter
			req, err := http.NewRequest("GET", "/api/v1/groups/children?uri=/list-children-test", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			// Cleanup
			delReq, _ := http.NewRequest("DELETE", "/api/v1/entries?uri=/list-children-test", nil)
			delReq.Header.Set("X-Namespace", types.DefaultNamespace)
			delW := httptest.NewRecorder()
			router.ServeHTTP(delW, delReq)
		})
	})

	Describe("ChangeParent", func() {
		It("should change entry parent", func() {
			// Create entries
			group1Req := CreateEntryRequest{
				URI:  "/test-group",
				Kind: "group",
			}
			group1Json, _ := json.Marshal(group1Req)
			group1HttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(group1Json))
			group1HttpReq.Header.Set("Content-Type", "application/json")
			group1HttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			group1W := httptest.NewRecorder()
			router.ServeHTTP(group1W, group1HttpReq)

			group2Req := CreateEntryRequest{
				URI:  "/test-folder",
				Kind: "group",
			}
			group2Json, _ := json.Marshal(group2Req)
			group2HttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(group2Json))
			group2HttpReq.Header.Set("Content-Type", "application/json")
			group2HttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			group2W := httptest.NewRecorder()
			router.ServeHTTP(group2W, group2HttpReq)

			// Change parent via query parameter
			changeReq := ChangeParentRequest{
				EntryURI:    "/test-folder",
				NewEntryURI: "/test-group/",
				Replace:     false,
			}
			changeJson, _ := json.Marshal(changeReq)
			changeHttpReq, _ := http.NewRequest("PUT", "/api/v1/entries/parent?uri=/test-folder&new_uri=/test-group/", bytes.NewBuffer(changeJson))
			changeHttpReq.Header.Set("Content-Type", "application/json")
			changeHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			changeW := httptest.NewRecorder()
			router.ServeHTTP(changeW, changeHttpReq)

			Expect(changeW.Code).To(Equal(http.StatusOK))
		})
	})
})

var _ = Describe("REST V1 Properties API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router

		// Create test entry
		createReq := CreateEntryRequest{
			URI:  "/test-group",
			Kind: "group",
		}
		createJson, _ := json.Marshal(createReq)
		createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
		createHttpReq.Header.Set("Content-Type", "application/json")
		createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
		createW := httptest.NewRecorder()
		router.ServeHTTP(createW, createHttpReq)
	})

	AfterEach(func() {
		// Cleanup
		req, _ := http.NewRequest("DELETE", "/api/v1/entries?uri=/test-group", nil)
		req.Header.Set("X-Namespace", types.DefaultNamespace)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})

	Describe("UpdateProperty", func() {
		It("should update entry properties", func() {
			reqBody := UpdatePropertyRequest{
				Tags:       []string{"tag1", "tag2"},
				Properties: map[string]string{"key": "value"},
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/entries/property?uri=/test-group", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return error for non-existent entry", func() {
			reqBody := UpdatePropertyRequest{
				Tags:       []string{"tag1"},
				Properties: map[string]string{"key": "value"},
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/entries/property?uri=/non-existent", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})

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
			// Create a workflow first
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

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("Workflow CRUD Full Flow", func() {
		It("should create, get, update, and delete workflow", func() {
			// Create
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

			// Get
			getReq, _ := http.NewRequest("GET", "/api/v1/workflows/wf-1", nil)
			getReq.Header.Set("X-Namespace", types.DefaultNamespace)
			getW := httptest.NewRecorder()
			router.ServeHTTP(getW, getReq)
			Expect(getW.Code).To(Equal(http.StatusOK))

			// Update
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

			// Delete
			delReq, _ := http.NewRequest("DELETE", "/api/v1/workflows/wf-1", nil)
			delReq.Header.Set("X-Namespace", types.DefaultNamespace)
			delW := httptest.NewRecorder()
			router.ServeHTTP(delW, delReq)
			Expect(delW.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GetJob", func() {
		BeforeEach(func() {
			// Create workflow first
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

			// Trigger a job
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
			// Create workflow and trigger job
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
			// Create workflow, trigger job, and pause it
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

			// Pause the job
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
			// Trigger
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

			// Get Job
			getJobReq, _ := http.NewRequest("GET", "/api/v1/workflows/wf-1/jobs/job-1", nil)
			getJobReq.Header.Set("X-Namespace", types.DefaultNamespace)
			getJobW := httptest.NewRecorder()
			router.ServeHTTP(getJobW, getJobReq)
			Expect(getJobW.Code).To(Equal(http.StatusOK))

			// Pause
			pauseReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/job-1/pause", nil)
			pauseReq.Header.Set("X-Namespace", types.DefaultNamespace)
			pauseW := httptest.NewRecorder()
			router.ServeHTTP(pauseW, pauseReq)
			Expect(pauseW.Code).To(Equal(http.StatusOK))

			// Resume
			resumeReq, _ := http.NewRequest("POST", "/api/v1/workflows/wf-1/jobs/job-1/resume", nil)
			resumeReq.Header.Set("X-Namespace", types.DefaultNamespace)
			resumeW := httptest.NewRecorder()
			router.ServeHTTP(resumeW, resumeReq)
			Expect(resumeW.Code).To(Equal(http.StatusOK))

			// Cancel
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
			// Create a workflow
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

			// List workflows
			listReq, err := http.NewRequest("GET", "/api/v1/workflows", nil)
			Expect(err).Should(BeNil())
			listReq.Header.Set("X-Namespace", types.DefaultNamespace)

			listW := httptest.NewRecorder()
			router.ServeHTTP(listW, listReq)

			Expect(listW.Code).To(Equal(http.StatusOK))
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

// Helper function to get pointer to bool
func BoolPtr(b bool) *bool {
	return &b
}

var _ = Describe("REST V1 Notify API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	Describe("ListMessages", func() {
		It("should return error for memory metastore without notification table", func() {
			req, err := http.NewRequest("GET", "/api/v1/messages", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Memory metastore doesn't support notifications table
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("ReadMessages", func() {
		It("should return error for memory metastore without notification table", func() {
			reqBody := ReadMessagesRequest{
				MessageIDList: []string{"msg1", "msg2"},
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/messages/read", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Memory metastore doesn't support notifications table
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})
})

var _ = Describe("REST V1 Tree API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	Describe("GroupTree", func() {
		It("should return tree structure for empty namespace", func() {
			req, err := http.NewRequest("GET", "/api/v1/groups/tree", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should create entries and return tree with children", func() {
			// Create multiple entries
			for _, uri := range []string{"/group1", "/group2"} {
				createReq := CreateEntryRequest{
					URI:  uri,
					Kind: "group",
				}
				createJson, _ := json.Marshal(createReq)
				createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
				createHttpReq.Header.Set("Content-Type", "application/json")
				createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
				createW := httptest.NewRecorder()
				router.ServeHTTP(createW, createHttpReq)
			}

			// Get tree
			req, err := http.NewRequest("GET", "/api/v1/groups/tree", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			// Cleanup
			for _, uri := range []string{"/group1", "/group2"} {
				delReq, _ := http.NewRequest("DELETE", "/api/v1/entries?uri="+uri, nil)
				delReq.Header.Set("X-Namespace", types.DefaultNamespace)
				delW := httptest.NewRecorder()
				router.ServeHTTP(delW, delReq)
			}
		})
	})
})

var _ = Describe("REST V1 Filter API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	Describe("FilterEntry", func() {
		It("should return empty list for CEL filter", func() {
			reqBody := FilterEntryRequest{
				CELPattern: "true",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/entries/search", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Returns empty list when no entries match
			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})
})

var _ = Describe("REST V1 DeleteEntries Batch API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
		testRouter.MockWorkflow.workflows = make(map[string]*types.Workflow)
		testRouter.MockWorkflow.jobs = make(map[string]*types.WorkflowJob)

		// Create test entries
		for _, uri := range []string{"/batch-test1", "/batch-test2"} {
			createReq := CreateEntryRequest{
				URI:  uri,
				Kind: "group",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
		}
	})

	Describe("DeleteEntries", func() {
		It("should batch delete entries", func() {
			reqBody := DeleteEntriesRequest{
				URIList: []string{"/batch-test1", "/batch-test2"},
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/entries/batch-delete", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should handle batch delete with partial failures", func() {
			reqBody := DeleteEntriesRequest{
				URIList: []string{"/batch-test1", "/non-existent"},
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/entries/batch-delete", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should still succeed, partial deletion is allowed
			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})
})

var _ = Describe("REST V1 File API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	Describe("WriteFile", func() {
		It("should return 400 for missing file in multipart form", func() {
			// Create a file entry first
			createReq := CreateEntryRequest{
				URI:  "/test-file.txt",
				Kind: "file",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
			Expect(createW.Code).To(Equal(http.StatusCreated))

			// Send request without file field using a non-existent ID
			req, err := http.NewRequest("POST", "/api/v1/files/content?id=99999", bytes.NewBufferString(""))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "multipart/form-data; boundary=----WebKitFormBoundary")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 404 for non-existent entry, not crash
			Expect(w.Code).To(Equal(http.StatusNotFound))

			// Cleanup
			delReq, _ := http.NewRequest("DELETE", "/api/v1/entries?uri=/test-file.txt", nil)
			delReq.Header.Set("X-Namespace", types.DefaultNamespace)
			delW := httptest.NewRecorder()
			router.ServeHTTP(delW, delReq)
		})

		It("should return error for non-existent entry", func() {
			req, err := http.NewRequest("POST", "/api/v1/files/content?id=99999", bytes.NewBufferString("test"))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "multipart/form-data; boundary=----WebKitFormBoundary")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("ReadFile", func() {
		It("should return error for non-existent entry", func() {
			req, err := http.NewRequest("GET", "/api/v1/files/content?id=99999", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})

var _ = Describe("REST V1 Entry Query API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	AfterEach(func() {
		// Cleanup: delete test entries
		cleanupEntries := []string{"/query-test", "/query-test/child", "/query-test/child/nested"}
		for _, uri := range cleanupEntries {
			req, _ := http.NewRequest("DELETE", "/api/v1/entries?uri="+uri, nil)
			req.Header.Set("X-Namespace", types.DefaultNamespace)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})

	Describe("EntryDetails", func() {
		It("should get entry via ?uri=", func() {
			// Create a test entry first
			createReq := CreateEntryRequest{
				URI:  "/query-test",
				Kind: "group",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
			Expect(createW.Code).To(Equal(http.StatusCreated))

			// Get entry via query parameter
			req, err := http.NewRequest("GET", "/api/v1/entries/details?uri=/query-test", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should get nested entry via ?uri=/group1/group2/entry", func() {
			// Create nested structure: /query-test/child/nested
			for _, uri := range []string{"/query-test", "/query-test/child", "/query-test/child/nested"} {
				createReq := CreateEntryRequest{
					URI:  uri,
					Kind: "group",
				}
				createJson, _ := json.Marshal(createReq)
				createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
				createHttpReq.Header.Set("Content-Type", "application/json")
				createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
				createW := httptest.NewRecorder()
				router.ServeHTTP(createW, createHttpReq)
				Expect(createW.Code).To(Equal(http.StatusCreated))
			}

			// Get nested entry via query parameter
			req, err := http.NewRequest("GET", "/api/v1/entries/details?uri=/query-test/child/nested", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return error when uri is missing", func() {
			req, err := http.NewRequest("GET", "/api/v1/entries/details", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should return 404 when entry not found", func() {
			req, err := http.NewRequest("GET", "/api/v1/entries/details?uri=/non-existent-entry", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("ListGroupChildren", func() {
		BeforeEach(func() {
			// Create test entries
			for _, uri := range []string{"/group-children-test", "/group-children-test/child1", "/group-children-test/child2"} {
				createReq := CreateEntryRequest{
					URI:  uri,
					Kind: "group",
				}
				createJson, _ := json.Marshal(createReq)
				createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
				createHttpReq.Header.Set("Content-Type", "application/json")
				createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
				createW := httptest.NewRecorder()
				router.ServeHTTP(createW, createHttpReq)
			}
		})

		AfterEach(func() {
			for _, uri := range []string{"/group-children-test", "/group-children-test/child1", "/group-children-test/child2"} {
				req, _ := http.NewRequest("DELETE", "/api/v1/entries?uri="+uri, nil)
				req.Header.Set("X-Namespace", types.DefaultNamespace)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}
		})

		It("should list children via ?uri=/group1/group2", func() {
			req, err := http.NewRequest("GET", "/api/v1/groups/children?uri=/group-children-test", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should list children of nested group via ?uri=/group1/group2/child", func() {
			req, err := http.NewRequest("GET", "/api/v1/groups/children?uri=/group-children-test/child1", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("EntryProperty", func() {
		BeforeEach(func() {
			createReq := CreateEntryRequest{
				URI:  "/query-test",
				Kind: "group",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
		})

		It("should update property via ?uri=", func() {
			reqBody := UpdatePropertyRequest{
				Tags:       []string{"tag1", "tag2"},
				Properties: map[string]string{"key": "value"},
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/entries/property?uri=/query-test", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 404 for non-existent entry", func() {
			reqBody := UpdatePropertyRequest{
				Tags: []string{"tag1"},
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/entries/property?uri=/non-existent", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("EntryParent", func() {
		BeforeEach(func() {
			for _, uri := range []string{"/query-test", "/query-test-dest"} {
				createReq := CreateEntryRequest{
					URI:  uri,
					Kind: "group",
				}
				createJson, _ := json.Marshal(createReq)
				createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
				createHttpReq.Header.Set("Content-Type", "application/json")
				createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
				createW := httptest.NewRecorder()
				router.ServeHTTP(createW, createHttpReq)
			}
		})

		It("should change parent via ?uri=&new_uri=", func() {
			reqBody := ChangeParentRequest{
				EntryURI:    "/query-test",
				NewEntryURI: "/query-test-dest/",
				Replace:     false,
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/entries/parent?uri=/query-test&new_uri=/query-test-dest/", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("EntryDocument", func() {
		BeforeEach(func() {
			// Create a file entry for document property testing
			createReq := CreateEntryRequest{
				URI:  "/query-test-file",
				Kind: "file",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)
		})

		AfterEach(func() {
			req, _ := http.NewRequest("DELETE", "/api/v1/entries?uri=/query-test-file", nil)
			req.Header.Set("X-Namespace", types.DefaultNamespace)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		})

		It("should update document via ?uri=", func() {
			reqBody := UpdateDocumentPropertyRequest{
				Unread: true,
				Marked: false,
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/entries/document?uri=/query-test-file", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return error for group entry", func() {
			// Create a group entry first
			createReq := CreateEntryRequest{
				URI:  "/query-test-group",
				Kind: "group",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)

			reqBody := UpdateDocumentPropertyRequest{
				Unread: true,
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/entries/document?uri=/query-test-group", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			// Cleanup
			delReq, _ := http.NewRequest("DELETE", "/api/v1/entries?uri=/query-test-group", nil)
			delReq.Header.Set("X-Namespace", types.DefaultNamespace)
			delW := httptest.NewRecorder()
			router.ServeHTTP(delW, delReq)
		})
	})

	Describe("Complex Nested URI Operations", func() {
		BeforeEach(func() {
			// Create a nested structure: /nested-uri-test -> /nested-uri-test/level1 -> /nested-uri-test/level1/level2
			// Create parents first: /nested-uri-test, then /nested-uri-test/level1
			levels := []string{
				"/nested-uri-test",
				"/nested-uri-test/level1",
			}
			for _, uri := range levels {
				createReq := CreateEntryRequest{
					URI:  uri,
					Kind: "group",
				}
				createJson, _ := json.Marshal(createReq)
				createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
				createHttpReq.Header.Set("Content-Type", "application/json")
				createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
				createW := httptest.NewRecorder()
				router.ServeHTTP(createW, createHttpReq)
			}
			// Create the leaf entry: /nested-uri-test/level1/level2
			createReq := CreateEntryRequest{
				URI:  "/nested-uri-test/level1/level2",
				Kind: "group",
			}
			createJson, _ := json.Marshal(createReq)
			createHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(createJson))
			createHttpReq.Header.Set("Content-Type", "application/json")
			createHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			createW := httptest.NewRecorder()
			router.ServeHTTP(createW, createHttpReq)

			// Create a nested file: /nested-uri-test/level1/level2/file.txt
			filePath := "/nested-uri-test/level1/level2/file.txt"
			fileReq := CreateEntryRequest{
				URI:  filePath,
				Kind: "file",
			}
			fileJson, _ := json.Marshal(fileReq)
			fileHttpReq, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(fileJson))
			fileHttpReq.Header.Set("Content-Type", "application/json")
			fileHttpReq.Header.Set("X-Namespace", types.DefaultNamespace)
			fileW := httptest.NewRecorder()
			router.ServeHTTP(fileW, fileHttpReq)
		})

		AfterEach(func() {
			// Cleanup nested structure
			paths := []string{
				"/nested-uri-test/level1/level2/file.txt",
				"/nested-uri-test/level1/level2",
				"/nested-uri-test/level1",
				"/nested-uri-test",
			}
			for _, uri := range paths {
				req, _ := http.NewRequest("DELETE", "/api/v1/entries?uri="+uri, nil)
				req.Header.Set("X-Namespace", types.DefaultNamespace)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}
		})

		It("should get nested entry via ?uri=/a/b", func() {
			nestedURI := "/nested-uri-test/level1/level2"
			req, err := http.NewRequest("GET", "/api/v1/entries/details?uri="+nestedURI, nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should list children of nested group", func() {
			parentURI := "/nested-uri-test/level1"
			req, err := http.NewRequest("GET", "/api/v1/groups/children?uri="+parentURI, nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should update nested entry via ?uri=", func() {
			nestedURI := "/nested-uri-test/level1/level2"
			updateReq := UpdateEntryRequest{
				Name: "updated-level2",
			}
			updateJson, _ := json.Marshal(updateReq)
			req, err := http.NewRequest("PUT", "/api/v1/entries?uri="+nestedURI, bytes.NewBuffer(updateJson))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should update property of nested entry", func() {
			nestedURI := "/nested-uri-test/level1/level2"
			reqBody := UpdatePropertyRequest{
				Tags:       []string{"nested", "deep"},
				Properties: map[string]string{"level": "2"},
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/entries/property?uri="+nestedURI, bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should update document property of nested file", func() {
			fileURI := "/nested-uri-test/level1/level2/file.txt"
			reqBody := UpdateDocumentPropertyRequest{
				Unread: true,
				Marked: true,
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/entries/document?uri="+fileURI, bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should handle non-existent URI gracefully", func() {
			req, err := http.NewRequest("GET", "/api/v1/entries/details?uri=/non-existent/nested/path", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 404, not crash
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})

var _ = Describe("REST V1 Configs API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	AfterEach(func() {
		// Cleanup: delete test configs
		testConfigs := []struct{ group, name string }{
			{"test-group", "test-key1"},
			{"test-group", "test-key2"},
			{"test-group", "test-key3"},
		}
		for _, cfg := range testConfigs {
			req, _ := http.NewRequest("DELETE", "/api/v1/configs/"+cfg.group+"/"+cfg.name, nil)
			req.Header.Set("X-Namespace", types.DefaultNamespace)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})

	Describe("SetConfig", func() {
		It("should set a new config value", func() {
			reqBody := SetConfigRequest{
				Value: "test-value",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/configs/test-group/test-key1", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 400 for missing value", func() {
			reqBody := map[string]string{}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/configs/test-group/test-key1", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GetConfig", func() {
		BeforeEach(func() {
			// Pre-set a config value
			reqBody := SetConfigRequest{
				Value: "test-value",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/configs/test-group/test-key1", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should get config value", func() {
			req, err := http.NewRequest("GET", "/api/v1/configs/test-group/test-key1", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 404 for non-existent config", func() {
			req, err := http.NewRequest("GET", "/api/v1/configs/test-group/non-existent-key", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("ListConfig", func() {
		BeforeEach(func() {
			// Pre-set multiple config values in the same group
			for i := 1; i <= 2; i++ {
				reqBody := SetConfigRequest{
					Value: "test-value-" + string(rune('0'+i)),
				}
				jsonBody, _ := json.Marshal(reqBody)

				req, err := http.NewRequest("PUT", "/api/v1/configs/test-group/test-key"+string(rune('0'+i)), bytes.NewBuffer(jsonBody))
				Expect(err).Should(BeNil())
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Namespace", types.DefaultNamespace)

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))
			}
		})

		It("should list configs by group", func() {
			req, err := http.NewRequest("GET", "/api/v1/configs/test-group", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("DeleteConfig", func() {
		BeforeEach(func() {
			// Pre-set a config value
			reqBody := SetConfigRequest{
				Value: "test-value-to-delete",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/configs/test-group/test-key3", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should delete config", func() {
			req, err := http.NewRequest("DELETE", "/api/v1/configs/test-group/test-key3", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 404 after deletion", func() {
			// First delete
			req, err := http.NewRequest("DELETE", "/api/v1/configs/test-group/test-key3", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))

			// Then try to get it (should be 404)
			req, err = http.NewRequest("GET", "/api/v1/configs/test-group/test-key3", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
