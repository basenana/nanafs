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
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	plugintypes "github.com/basenana/plugin/types"
)

var _ = Describe("REST V1 Plugin API", func() {
	var router *gin.Engine

	BeforeEach(func() {
		router = testRouter.Router
	})

	Describe("ListPlugins", func() {
		It("should return empty plugins list when no plugins registered", func() {
			req, err := http.NewRequest("GET", "/api/v1/workflows/plugins", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", "default")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return registered plugins with parameters", func() {
			testRouter.MockWorkflow.AddPlugin(plugintypes.PluginSpec{
				Name:    "archive",
				Version: "1.0",
				Type:    plugintypes.TypeProcess,
				Parameters: []plugintypes.ParameterSpec{
					{
						Name:        "action",
						Required:    false,
						Default:     "extract",
						Description: "Action to perform",
						Options:     []string{"extract", "compress"},
					},
					{
						Name:     "format",
						Required: true,
						Options:  []string{"zip", "tar", "gzip"},
					},
				},
			})

			testRouter.MockWorkflow.AddPlugin(plugintypes.PluginSpec{
				Name:    "checksum",
				Version: "1.0",
				Type:    plugintypes.TypeProcess,
				Parameters: []plugintypes.ParameterSpec{
					{
						Name:     "algorithm",
						Required: false,
						Default:  "md5",
						Options:  []string{"md5", "sha256"},
					},
				},
			})

			req, err := http.NewRequest("GET", "/api/v1/workflows/plugins", nil)
			Expect(err).Should(BeNil())
			req.Header.Set("X-Namespace", "default")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.Len()).To(BeNumerically(">", 0))
		})
	})
})
