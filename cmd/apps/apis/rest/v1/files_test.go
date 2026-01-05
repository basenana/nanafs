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

var _ = Describe("REST V1 File API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	Describe("WriteFile", func() {
		It("should return 400 for missing file in multipart form", func() {
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

			req, err := http.NewRequest("POST", "/api/v1/files/content?id=99999", bytes.NewBufferString(""))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "multipart/form-data; boundary=----WebKitFormBoundary")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))

			delReq, _ := http.NewRequest("DELETE", "/api/v1/entries?uri=/test-file.txt", nil)
			delReq.Header.Set("X-Namespace", types.DefaultNamespace)
			delW := httptest.NewRecorder()
			router.ServeHTTP(delW, delReq)
		})

		It("should return 404 for non-existent entry", func() {
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
		It("should return 404 for non-existent entry", func() {
			req, err := http.NewRequest("GET", "/api/v1/files/content?id=99999", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
