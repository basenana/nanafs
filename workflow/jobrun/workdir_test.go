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

package jobrun

import (
	"context"
	"os"
	"path/filepath"

	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("initWorkdir", func() {
	var (
		ctx     context.Context
		job     *types.WorkflowJob
		workdir string
	)

	BeforeEach(func() {
		ctx = context.Background()
		job = newWorkflowJob("init-test")

		var err error
		workdir, err = os.MkdirTemp(tempDir, "init-workdir-")
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		if workdir != "" {
			os.RemoveAll(workdir)
		}
	})

	Context("with non-existent directory", func() {
		It("should create workdir successfully", func() {
			newWorkdir := filepath.Join(workdir, "new-job-workdir")

			err := initWorkdir(ctx, newWorkdir, job)

			Expect(err).To(BeNil())
			Expect(newWorkdir).To(BeADirectory())
		})

		It("should create workflowinfo.json file", func() {
			newWorkdir := filepath.Join(workdir, "new-job-workdir")

			err := initWorkdir(ctx, newWorkdir, job)

			Expect(err).To(BeNil())
			infoFile := filepath.Join(newWorkdir, ".workflowinfo.json")
			Expect(infoFile).To(BeAnExistingFile())
		})
	})

	Context("with existing directory", func() {
		It("should return nil without error", func() {
			newWorkdir := filepath.Join(workdir, "existing-dir")
			err := os.MkdirAll(newWorkdir, 0755)
			Expect(err).To(BeNil())

			err = initWorkdir(ctx, newWorkdir, job)

			Expect(err).To(BeNil())
		})
	})

	Context("with path exists as file", func() {
		It("should return error", func() {
			filePath := filepath.Join(workdir, "existing-file")
			err := os.WriteFile(filePath, []byte("content"), 0644)
			Expect(err).To(BeNil())

			err = initWorkdir(ctx, filePath, job)

			Expect(err).ToNot(BeNil())
		})
	})
})

var _ = Describe("cleanupWorkdir", func() {
	var (
		ctx     context.Context
		workdir string
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	AfterEach(func() {
		if workdir != "" {
			os.RemoveAll(workdir)
		}
	})

	Context("with existing workdir", func() {
		It("should remove workdir successfully", func() {
			var err error
			workdir, err = os.MkdirTemp(tempDir, "cleanup-workdir-")
			Expect(err).To(BeNil())

			err = cleanupWorkdir(ctx, workdir)
			Expect(err).To(BeNil())
			Expect(workdir).ToNot(BeADirectory())
		})
	})

	Context("with empty workdir path", func() {
		It("should return nil without error", func() {
			err := cleanupWorkdir(ctx, "")
			Expect(err).To(BeNil())
		})
	})

	Context("with non-existent workdir", func() {
		It("should return nil without error", func() {
			nonExistent := filepath.Join(tempDir, "non-existent-dir")

			err := cleanupWorkdir(ctx, nonExistent)

			Expect(err).To(BeNil())
		})
	})

	Context("with path exists as file", func() {
		It("should return error", func() {
			filePath := filepath.Join(tempDir, "not-a-dir")
			err := os.WriteFile(filePath, []byte("content"), 0644)
			Expect(err).To(BeNil())
			workdir = filePath

			err = cleanupWorkdir(ctx, workdir)

			Expect(err).ToNot(BeNil())
		})
	})
})
