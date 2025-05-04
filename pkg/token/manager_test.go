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

package token

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("testAccessToken", func() {
	var (
		ctx = context.TODO()
	)

	Context("test token not exist", func() {
		It("access token should be failed", func() {
			_, err := manager.AccessToken(ctx, "ak-unknown", "sk-unknown")
			Expect(err).ShouldNot(BeNil())
		})
	})

	Context("test token exist", func() {
		const (
			ak = "ak-mocked-1"
			sk = "sk-mocked-1"
			ns = "test-ns"
		)
		It("insert access token should be succeed", func() {
			token := &types.AccessToken{
				TokenKey:    ak,
				SecretToken: sk,
				Namespace:   "personal",
			}
			err := testMeta.CreateAccessToken(ctx, ns, token)
			Expect(err).Should(BeNil())
		})
		It("access token should be failed", func() {
			token, err := manager.AccessToken(ctx, ak, sk)
			Expect(err).Should(BeNil())

			Expect(len(token.ClientCrt) > 0).Should(BeTrue())
			Expect(len(token.ClientKey) > 0).Should(BeTrue())
			Expect(time.Now().Before(token.CertExpiration)).Should(BeTrue())
		})

		It("access token again should be failed", func() {
			token, err := manager.AccessToken(ctx, ak, sk)
			Expect(err).Should(BeNil())

			Expect(len(token.ClientCrt) > 0).Should(BeTrue())
			Expect(len(token.ClientKey) > 0).Should(BeTrue())
			Expect(time.Now().Before(token.CertExpiration)).Should(BeTrue())
		})
	})
})
