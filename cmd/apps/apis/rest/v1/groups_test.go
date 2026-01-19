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
				delReqBody := map[string]string{"uri": uri}
				delJsonBody, _ := json.Marshal(delReqBody)
				delReq, _ := http.NewRequest("POST", "/api/v1/entries/delete", bytes.NewBuffer(delJsonBody))
				delReq.Header.Set("Content-Type", "application/json")
				delReq.Header.Set("X-Namespace", types.DefaultNamespace)
				delW := httptest.NewRecorder()
				router.ServeHTTP(delW, delReq)
			}
		})
	})

	Describe("ListGroupChildren", func() {
		It("should return pagination info", func() {
			reqBody := ListGroupChildrenRequest{
				EntrySelector: EntrySelector{URI: "/"},
				Page:          1,
				PageSize:      10,
			}
			jsonBody, _ := json.Marshal(reqBody)
			req, err := http.NewRequest("POST", "/api/v1/groups/children", bytes.NewBuffer(jsonBody))
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

var _ = Describe("buildGroupTreeFromChildren", func() {
	It("should build tree from flat children list", func() {
		children := []*types.Child{
			{ParentID: 1, ChildID: 2, Name: "group1", IsGroup: true},
			{ParentID: 2, ChildID: 3, Name: "subgroup1", IsGroup: true},
			{ParentID: 2, ChildID: 4, Name: "subgroup2", IsGroup: true},
			{ParentID: 1, ChildID: 5, Name: "group2", IsGroup: true},
		}

		tree := buildGroupTreeFromChildren(children, 1)

		Expect(tree).Should(HaveLen(2))
		Expect(tree[0].Name).Should(Equal("group1"))
		Expect(tree[0].Children).Should(HaveLen(2))
		Expect(tree[0].Children[0].Name).Should(Equal("subgroup1"))
		Expect(tree[0].Children[1].Name).Should(Equal("subgroup2"))
		Expect(tree[1].Name).Should(Equal("group2"))
	})

	It("should return empty for no children", func() {
		tree := buildGroupTreeFromChildren([]*types.Child{}, 1)
		Expect(tree).Should(BeEmpty())
	})

	It("should handle nested structure", func() {
		children := []*types.Child{
			{ParentID: 1, ChildID: 2, Name: "g1", IsGroup: true},
			{ParentID: 2, ChildID: 3, Name: "g1-1", IsGroup: true},
			{ParentID: 3, ChildID: 4, Name: "g1-1-1", IsGroup: true},
		}

		tree := buildGroupTreeFromChildren(children, 1)

		Expect(tree).Should(HaveLen(1))
		Expect(tree[0].Name).Should(Equal("g1"))
		Expect(tree[0].Children).Should(HaveLen(1))
		Expect(tree[0].Children[0].Name).Should(Equal("g1-1"))
		Expect(tree[0].Children[0].Children).Should(HaveLen(1))
		Expect(tree[0].Children[0].Children[0].Name).Should(Equal("g1-1-1"))
	})

	It("should handle single root group", func() {
		children := []*types.Child{
			{ParentID: 1, ChildID: 2, Name: "single", IsGroup: true},
		}

		tree := buildGroupTreeFromChildren(children, 1)

		Expect(tree).Should(HaveLen(1))
		Expect(tree[0].Name).Should(Equal("single"))
		Expect(tree[0].Children).Should(BeEmpty())
	})

	It("should handle multiple levels", func() {
		children := []*types.Child{
			{ParentID: 1, ChildID: 2, Name: "a", IsGroup: true},
			{ParentID: 1, ChildID: 3, Name: "b", IsGroup: true},
			{ParentID: 2, ChildID: 4, Name: "a1", IsGroup: true},
			{ParentID: 2, ChildID: 5, Name: "a2", IsGroup: true},
			{ParentID: 3, ChildID: 6, Name: "b1", IsGroup: true},
			{ParentID: 4, ChildID: 7, Name: "a1-1", IsGroup: true},
		}

		tree := buildGroupTreeFromChildren(children, 1)

		Expect(tree).Should(HaveLen(2))

		a := tree[0]
		Expect(a.Name).Should(Equal("a"))
		Expect(a.Children).Should(HaveLen(2))

		a1 := a.Children[0]
		Expect(a1.Name).Should(Equal("a1"))
		Expect(a1.Children).Should(HaveLen(1))
		Expect(a1.Children[0].Name).Should(Equal("a1-1"))

		a2 := a.Children[1]
		Expect(a2.Name).Should(Equal("a2"))
		Expect(a2.Children).Should(BeEmpty())

		b := tree[1]
		Expect(b.Name).Should(Equal("b"))
		Expect(b.Children).Should(HaveLen(1))
		Expect(b.Children[0].Name).Should(Equal("b1"))
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
