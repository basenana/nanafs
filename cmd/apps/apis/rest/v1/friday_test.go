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

var _ = Describe("REST V1 Friday Session API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	Describe("CreateSession", func() {
		It("should return 503 when LLM is not enabled", func() {
			reqBody := CreateSessionRequest{
				Name: "test-session",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/friday/sessions", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
		})
	})

	Describe("ListSessions", func() {
		It("should return 503 when LLM is not enabled", func() {
			req, err := http.NewRequest("GET", "/api/v1/friday/sessions", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
		})
	})

	Describe("GetSession", func() {
		It("should return 503 when LLM is not enabled", func() {
			req, err := http.NewRequest("GET", "/api/v1/friday/sessions/test-session-id", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
		})
	})

	Describe("DeleteSession", func() {
		It("should return 503 when LLM is not enabled", func() {
			req, err := http.NewRequest("DELETE", "/api/v1/friday/sessions/test-session-id", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
		})
	})

	Describe("RenameSession", func() {
		It("should return 503 when LLM is not enabled", func() {
			reqBody := RenameSessionRequest{
				Name: "new-name",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("PUT", "/api/v1/friday/sessions/test-session-id", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
		})
	})

	Describe("Chat", func() {
		It("should return 503 when LLM is not enabled", func() {
			reqBody := ChatRequest{
				Message: "hello",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/friday/chat", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
		})

		It("should return 503 when LLM is not enabled (missing message)", func() {
			reqBody := ChatRequest{}
			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", "/api/v1/friday/chat", bytes.NewBuffer(jsonBody))
			Expect(err).Should(BeNil())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
		})
	})
})
