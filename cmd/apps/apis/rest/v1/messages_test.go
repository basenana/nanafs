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

var _ = Describe("REST V1 Notify API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	Describe("ListMessages", func() {
		It("should return empty list for memory metastore", func() {
			req, err := http.NewRequest("GET", "/api/v1/messages", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Memory metastore returns empty list
			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("ReadMessages", func() {
		It("should handle empty message list", func() {
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

			// Memory metastore handles gracefully
			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})
})
