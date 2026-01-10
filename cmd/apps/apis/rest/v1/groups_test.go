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

			req, err := http.NewRequest("GET", "/api/v1/groups/tree", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			for _, uri := range []string{"/group1", "/group2"} {
				delReq, _ := http.NewRequest("DELETE", "/api/v1/entries?uri="+uri, nil)
				delReq.Header.Set("X-Namespace", types.DefaultNamespace)
				delW := httptest.NewRecorder()
				router.ServeHTTP(delW, delReq)
			}
		})
	})

	Describe("ListGroupChildren", func() {
		It("should return pagination info", func() {
			req, err := http.NewRequest("GET", "/api/v1/groups/children?uri=/&page=1&page_size=10", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp ListEntriesResponse
			err = json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).Should(BeNil())
			Expect(resp.Pagination).ShouldNot(BeNil())
			Expect(resp.Pagination.Page).To(Equal(int64(1)))
			Expect(resp.Pagination.PageSize).To(Equal(int64(10)))
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

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})
})
