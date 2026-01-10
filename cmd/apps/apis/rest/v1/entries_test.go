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

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})
	})

	Describe("GetEntryDetail", func() {
		It("should get entry detail after creation", func() {
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
		It("should update entry aliases", func() {
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

			updateReq := UpdateEntryRequest{
				Aliases: "updated-alias",
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

			req, err := http.NewRequest("GET", "/api/v1/groups/children?uri=/list-children-test", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			delReq, _ := http.NewRequest("DELETE", "/api/v1/entries?uri=/list-children-test", nil)
			delReq.Header.Set("X-Namespace", types.DefaultNamespace)
			delW := httptest.NewRecorder()
			router.ServeHTTP(delW, delReq)
		})
	})

	Describe("ChangeParent", func() {
		It("should change entry parent", func() {
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

		It("should return 404 for non-existent entry", func() {
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

		It("should return pagination info", func() {
			reqBody := FilterEntryRequest{
				CELPattern: "true",
				Page:       1,
				PageSize:   10,
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/entries/search", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
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

var _ = Describe("REST V1 Entry Query API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router

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
		Expect(createW.Code).To(Equal(http.StatusCreated))
	})

	AfterEach(func() {
		req, _ := http.NewRequest("DELETE", "/api/v1/entries?uri=/query-test-group", nil)
		req.Header.Set("X-Namespace", types.DefaultNamespace)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})

	Describe("EntryDetails", func() {
		It("should return entry details using query parameter", func() {
			req, err := http.NewRequest("GET", "/api/v1/entries/details?uri=/query-test-group", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
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

	Describe("EntryChildren", func() {
		It("should return entry children", func() {
			req, err := http.NewRequest("GET", "/api/v1/entries/details?uri=/query-test-group", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("Complex Nested URI Operations", func() {
		It("should handle non-existent URI gracefully", func() {
			req, err := http.NewRequest("GET", "/api/v1/entries/details?uri=/non-existent/uri/path", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
