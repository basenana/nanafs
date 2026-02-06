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
	"github.com/basenana/nanafs/utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Executor Setup", func() {
	var (
		ctx context.Context
		job *types.WorkflowJob
	)

	BeforeEach(func() {
		ctx = context.Background()
		job = newWorkflowJob("setup-test")
	})

	Context("with empty targets", func() {
		It("should return error for empty targets", func() {
			job.Targets.Entries = nil

			exec := &defaultExecutor{
				job:    job,
				logger: logger.NewLogger("test"),
			}

			err := exec.Setup(ctx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("no targets"))
		})
	})
})

var _ = Describe("Executor Exec", func() {
	var (
		ctx  context.Context
		job  *types.WorkflowJob
		exec *defaultExecutor
	)

	BeforeEach(func() {
		ctx = context.Background()
		job = newWorkflowJob("exec-test")
		exec = &defaultExecutor{
			job:     job,
			logger:  logger.NewLogger("test"),
			results: NewMemBasedResults(),
		}
	})

	Context("with condition node - true branch", func() {
		It("should set branch next to true branch", func() {
			conditionStep := newConditionNode("condition-step", "true", map[string]string{"true": "branch-true", "false": "branch-false"})
			task := &Task{job: job, step: &conditionStep}

			err := exec.Exec(ctx, nil, task)
			Expect(err).To(BeNil())
			Expect(task.GetBranchNext()).To(Equal("branch-true"))
		})
	})

	Context("with condition node - false branch", func() {
		It("should set branch next to false branch", func() {
			conditionStep := newConditionNode("condition-step", "false", map[string]string{"true": "branch-true", "false": "branch-false"})
			task := &Task{job: job, step: &conditionStep}

			err := exec.Exec(ctx, nil, task)
			Expect(err).To(BeNil())
			Expect(task.GetBranchNext()).To(Equal("branch-false"))
		})
	})
})

var _ = Describe("Executor Teardown", func() {
	var (
		ctx  context.Context
		exec *defaultExecutor
	)

	BeforeEach(func() {
		ctx = context.Background()
		job := newWorkflowJob("teardown-test")
		exec = &defaultExecutor{
			job:     job,
			workdir: filepath.Join(tempDir, "test-workdir"),
			logger:  logger.NewLogger("test"),
		}
	})

	Context("with existing workdir", func() {
		It("should cleanup workdir successfully", func() {
			err := os.MkdirAll(exec.workdir, 0755)
			Expect(err).To(BeNil())

			err = exec.Teardown(ctx)
			Expect(err).To(BeNil())
			Expect(exec.workdir).ToNot(BeAnExistingFile())
		})
	})

	Context("with non-existent workdir", func() {
		It("should return nil without error", func() {
			exec.workdir = filepath.Join(tempDir, "non-existent-workdir")

			err := exec.Teardown(ctx)
			Expect(err).To(BeNil())
		})
	})
})
