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

var _ = Describe("REST V1 Configs API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	AfterEach(func() {
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
			req, err := http.NewRequest("DELETE", "/api/v1/configs/test-group/test-key3", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))

			req, err = http.NewRequest("GET", "/api/v1/configs/test-group/test-key3", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", types.DefaultNamespace)

			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
