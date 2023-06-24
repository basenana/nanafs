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

package storage

import (
	"bytes"
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestEncrypt", func() {
	Context("test short data", func() {
		data, err := utils.RandString(1024)
		Expect(err).Should(BeNil())
		sKey, err := utils.RandString(16)
		Expect(err).Should(BeNil())

		var encodedData []byte
		It("encrypt should be succeed", func() {
			result := &bytes.Buffer{}
			err = encrypt(context.TODO(), 101, config.AESEncryption, sKey, bytes.NewReader([]byte(data)), result)
			Expect(err).Should(BeNil())
			encodedData = result.Bytes()
		})

		It("decrypt should not be succeed", func() {
			result := &bytes.Buffer{}
			err = decrypt(context.TODO(), 102, config.AESEncryption, sKey, bytes.NewReader(encodedData), result)
			Expect(err).Should(BeNil())
			Expect(result.Bytes()).ShouldNot(Equal([]byte(data)))

			result.Reset()
			fakeSk, err := utils.RandString(16)
			Expect(err).Should(BeNil())
			err = decrypt(context.TODO(), 101, config.AESEncryption, fakeSk, bytes.NewReader(encodedData), result)
			Expect(err).Should(BeNil())
			Expect(result.Bytes()).ShouldNot(Equal([]byte(data)))
		})

		It("decrypt should be succeed", func() {
			result := &bytes.Buffer{}
			err = decrypt(context.TODO(), 101, config.AESEncryption, sKey, bytes.NewReader(encodedData), result)
			Expect(err).Should(BeNil())
			Expect(result.Bytes()).Should(Equal([]byte(data)))
		})
	})

	Context("test long data", func() {
		data, err := utils.RandString(cacheNodeBaseSize * 16)
		Expect(err).Should(BeNil())
		sKey, err := utils.RandString(32)
		Expect(err).Should(BeNil())

		var encodedData []byte
		It("encrypt should be succeed", func() {
			result := &bytes.Buffer{}
			err = encrypt(context.TODO(), 101, config.AESEncryption, sKey, bytes.NewReader([]byte(data)), result)
			Expect(err).Should(BeNil())
			encodedData = result.Bytes()
		})

		It("decrypt should not be succeed", func() {
			result := &bytes.Buffer{}
			err = decrypt(context.TODO(), 102, config.AESEncryption, sKey, bytes.NewReader(encodedData), result)
			Expect(err).Should(BeNil())
			Expect(result.Bytes()).ShouldNot(Equal([]byte(data)))

			result.Reset()
			fakeSk, err := utils.RandString(16)
			Expect(err).Should(BeNil())
			err = decrypt(context.TODO(), 101, config.AESEncryption, fakeSk, bytes.NewReader(encodedData), result)
			Expect(err).Should(BeNil())
			Expect(result.Bytes()).ShouldNot(Equal([]byte(data)))
		})

		It("decrypt should be succeed", func() {
			result := &bytes.Buffer{}
			err = decrypt(context.TODO(), 101, config.AESEncryption, sKey, bytes.NewReader(encodedData), result)
			Expect(err).Should(BeNil())
			Expect(result.Bytes()).Should(Equal([]byte(data)))
		})
	})
})
