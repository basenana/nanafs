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

package workflow

import (
	"bytes"
	"context"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	"io"
	"time"
)

var _ = Describe("TestMirrorPlugin", func() {
	var (
		ctx         = context.TODO()
		workflowDir dentry.Entry
		jobDir      dentry.Entry

		workflowID1 = "test-workflow-1"
		workflowID2 = "test-workflow-delete"
		workflow1   dentry.Entry
		workflow2   dentry.Entry
	)
	Context("init mirror plugin", func() {
		It("list root should has jobs and workflows", func() {
			root, err := entryMgr.Root(ctx)
			Expect(err).Should(BeNil())

			child, err := root.Group().ListChildren(ctx)
			Expect(err).Should(BeNil())

			var workflowRoot dentry.Entry
			for i, dir := range child {
				if dir.Metadata().Name == MirrorRootDirName {
					workflowRoot = child[i]
					break
				}
			}
			Expect(workflowRoot).ShouldNot(BeNil())

			child, err = workflowRoot.Group().ListChildren(ctx)
			Expect(err).Should(BeNil())
			for i, ch := range child {
				md := ch.Metadata()
				switch md.Name {
				case MirrorDirWorkflows:
					workflowDir = child[i]
				case MirrorDirJobs:
					jobDir = child[i]
				}
			}
			Expect(workflowDir).ShouldNot(BeNil())
			Expect(jobDir).ShouldNot(BeNil())
		})
	})
	Context("manage workflow", func() {
		It("create workflow should be succeed", func() {
			var err error
			workflow1, err = entryMgr.CreateEntry(ctx, workflowDir, dentry.EntryAttr{
				Name:   id2MirrorFile(workflowID1),
				Kind:   types.RawKind,
				Access: workflowDir.Metadata().Access,
			})
			Expect(err).Should(BeNil())

			f, err := entryMgr.Open(ctx, workflow1, dentry.Attr{Write: true})
			Expect(err).Should(BeNil())

			_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, f), bytes.NewBuffer([]byte(workflowPayload1)))
			Expect(err).Should(BeNil())

			Expect(f.Close(ctx)).Should(BeNil())

			_, err = mgr.GetWorkflow(ctx, workflowID1)
			Expect(err).Should(BeNil())

			workflow2, err = entryMgr.CreateEntry(ctx, workflowDir, dentry.EntryAttr{
				Name:   id2MirrorFile(workflowID2),
				Kind:   types.RawKind,
				Access: workflowDir.Metadata().Access,
			})
			Expect(err).Should(BeNil())

			f, err = entryMgr.Open(ctx, workflow2, dentry.Attr{Write: true})
			Expect(err).Should(BeNil())

			_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, f), bytes.NewBuffer([]byte(workflowPayload1)))
			Expect(err).Should(BeNil())

			Expect(f.Close(ctx)).Should(BeNil())

			_, err = mgr.GetWorkflow(ctx, workflowID2)
			Expect(err).Should(BeNil())
		})
		It("update workflow should be succeed", func() {
			f, err := entryMgr.Open(ctx, workflow1, dentry.Attr{Write: true, Trunc: true})
			Expect(err).Should(BeNil())

			_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, f), bytes.NewBuffer([]byte(workflowPayload2)))
			Expect(err).Should(BeNil())

			Expect(f.Close(ctx)).Should(BeNil())

			_, err = mgr.GetWorkflow(ctx, workflowID1)
			Expect(err).Should(BeNil())
		})
		It("delete workflow should be succeed", func() {
			_, err := mgr.GetWorkflow(ctx, workflowID2)
			Expect(err).Should(BeNil())

			err = workflowDir.Group().RemoveEntry(ctx, workflow2)
			Expect(err).Should(BeNil())

			_, err = mgr.GetWorkflow(ctx, workflowID2)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})

	var (
		jobID               = "test-workflow-1-job-1"
		workflowJob         dentry.Entry
		jobWorkflowGroupDir dentry.Entry

		targetEn dentry.Entry
	)
	Context("trigger workflow", func() {
		It("create dummy entry should be succeed", func() {
			root, err := entryMgr.Root(ctx)
			Expect(err).Should(BeNil())
			targetEn, err = entryMgr.CreateEntry(ctx, root, dentry.EntryAttr{Name: "test_workflow_mirror_dir.txt", Kind: types.RawKind, Access: root.Metadata().Access})
			Expect(err).Should(BeNil())

			f, err := entryMgr.Open(ctx, targetEn, dentry.Attr{Write: true})
			Expect(err).Should(BeNil())
			_, err = f.WriteAt(ctx, []byte("Hello World!"), 0)
			Expect(err).Should(BeNil())
			Expect(f.Close(ctx)).Should(BeNil())
		})
		It("trigger workflow should be succeed", func() {
			var err error

			// get job workflow group
			jobWorkflowGroupDir, err = jobDir.Group().FindEntry(ctx, workflowID1)
			Expect(err).Should(BeNil())

			// trigger workflow
			workflowJob, err = entryMgr.CreateEntry(ctx, jobWorkflowGroupDir, dentry.EntryAttr{
				Name:   id2MirrorFile(jobID),
				Kind:   types.RawKind,
				Access: workflowDir.Metadata().Access,
			})
			Expect(err).Should(BeNil())

			f, err := entryMgr.Open(ctx, workflowJob, dentry.Attr{Write: true})
			Expect(err).Should(BeNil())

			job := &types.WorkflowJob{
				TriggerReason: "unit test",
				Target:        types.WorkflowTarget{EntryID: targetEn.Metadata().ID},
			}
			jobContent, err := yaml.Marshal(job)
			Expect(err).Should(BeNil())

			_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, f), bytes.NewBuffer(jobContent))
			Expect(err).Should(BeNil())

			Expect(f.Close(ctx)).Should(BeNil())

			// query job
			jobs, err := mgr.ListJobs(ctx, workflowID1)
			Expect(err).Should(BeNil())
			Expect(len(jobs)).Should(Equal(1))

			Expect(jobs[0].Id).Should(Equal(jobID))
		})
		It("pause workflow job should be succeed", func() {
			jobs, err := mgr.ListJobs(ctx, workflowID1)
			Expect(err).Should(BeNil())
			Expect(len(jobs)).Should(Equal(1))

			job := jobs[0]
			job.Status = flow.PausedStatus

			f, err := entryMgr.Open(ctx, workflowJob, dentry.Attr{Write: true, Trunc: true})
			Expect(err).Should(BeNil())

			encoder := yaml.NewEncoder(utils.NewWriterWithContextWriter(ctx, f))
			err = encoder.Encode(job)
			Expect(err).Should(BeNil())
			Expect(f.Close(ctx)).Should(BeNil())

			Eventually(func() string {
				jobs, err := mgr.ListJobs(ctx, workflowID1)
				Expect(err).Should(BeNil())
				Expect(len(jobs)).Should(Equal(1))
				job := jobs[0]
				return job.Status
			}, time.Minute, time.Second).Should(Equal(flow.PausedStatus))
		})
		It("resume workflow job should be succeed", func() {
			jobs, err := mgr.ListJobs(ctx, workflowID1)
			Expect(err).Should(BeNil())
			Expect(len(jobs)).Should(Equal(1))

			job := jobs[0]
			job.Status = flow.RunningStatus

			f, err := entryMgr.Open(ctx, workflowJob, dentry.Attr{Write: true, Trunc: true})
			Expect(err).Should(BeNil())

			encoder := yaml.NewEncoder(utils.NewWriterWithContextWriter(ctx, f))
			err = encoder.Encode(job)
			Expect(err).Should(BeNil())
			Expect(f.Close(ctx)).Should(BeNil())

			Eventually(func() string {
				jobs, err := mgr.ListJobs(ctx, workflowID1)
				Expect(err).Should(BeNil())
				Expect(len(jobs)).Should(Equal(1))
				job := jobs[0]
				return job.Status
			}, time.Minute, time.Second).Should(Equal(flow.RunningStatus))
		})
		It("workflow job should finish", func() {
			Eventually(func() string {
				jobs, err := mgr.ListJobs(ctx, workflowID1)
				Expect(err).Should(BeNil())
				Expect(len(jobs)).Should(Equal(1))
				job := jobs[0]
				return job.Status
			}, time.Minute, time.Second).Should(Equal(flow.SucceedStatus))
		})
	})
})

var (
	workflowPayload1 = ``
	workflowPayload2 = ``
)

func init() {
	var (
		ps = &types.PlugScope{
			PluginName: "delay",
			Version:    "1.0",
			PluginType: types.TypeProcess,
			Action:     "delay",
			Parameters: map[string]string{"delay": "10s"},
		}
		wf = &types.WorkflowSpec{
			Name: "test-workflow-mirror-dir-1",
			Rule: types.Rule{},
			Steps: []types.WorkflowStepSpec{
				{Name: "step-1", Plugin: ps},
				{Name: "step-2", Plugin: ps},
				{Name: "step-3", Plugin: ps},
			},
		}
	)

	wfContent1, err := yaml.Marshal(wf)
	if err != nil {
		panic(err)
	}
	workflowPayload1 = string(wfContent1)

	wf.Steps = []types.WorkflowStepSpec{
		{Name: "step-1", Plugin: ps},
		{Name: "step-2", Plugin: ps},
		{Name: "step-3", Plugin: ps},
		{Name: "step-4", Plugin: ps},
	}
	wfContent2, err := yaml.Marshal(wf)
	if err != nil {
		panic(err)
	}
	workflowPayload2 = string(wfContent2)
}
