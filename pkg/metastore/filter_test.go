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

package metastore

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("TestSqliteFileFilter", func() {
	var sqlite = buildNewSqliteMetaStore("test_filter.db")
	// init root
	rootEn := InitRootEntry()
	Expect(sqlite.CreateEntry(context.TODO(), namespace, 0, rootEn)).Should(BeNil())

	ctx := context.TODO()
	Context("filter file entry", func() {
		var (
			file1, file2 *types.Entry
			err          error
		)

		It("create file with tags should be succeed", func() {
			file1, err = types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-new-file-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, file1)).Should(BeNil())
			Expect(sqlite.UpdateEntryProperties(context.TODO(), namespace, types.PropertyTypeProperty, file1.ID, types.Properties{
				Tags:       []string{"tag1", "tag2"},
				URL:        "https://test2.com/abc",
				SiteName:   "test",
				Properties: map[string]string{"key": "value"},
			})).Should(BeNil())

			file2, err = types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-new-file-2",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, file2)).Should(BeNil())
			Expect(sqlite.UpdateEntryProperties(context.TODO(), namespace, types.PropertyTypeProperty, file2.ID, types.Properties{
				Tags:       []string{"tag2", "tag3"},
				URL:        "https://test.com/abc",
				SiteName:   "test",
				Properties: map[string]string{"key": "value"},
			})).Should(BeNil())
		})
		It("filter file using tag should be succeed", func() {
			it, err := sqlite.FilterEntries(ctx, namespace, types.Filter{CELPattern: `"tag1" in tags`})
			Expect(err).Should(BeNil())

			hasTag := make(map[int64]struct{})
			for it.HasNext() {
				next, err := it.Next()
				Expect(err).Should(BeNil())
				hasTag[next.ID] = struct{}{}
			}

			_, ok := hasTag[file1.ID]
			Expect(ok).Should(BeTrue())
		})
		It("filter file using tag should be succeed", func() {
			it, err := sqlite.FilterEntries(ctx, namespace, types.Filter{CELPattern: `url.contains("test.com")`})
			Expect(err).Should(BeNil())

			hasTag := make(map[int64]struct{})
			for it.HasNext() {
				next, err := it.Next()
				Expect(err).Should(BeNil())
				hasTag[next.ID] = struct{}{}
			}

			_, ok := hasTag[file2.ID]
			Expect(ok).Should(BeTrue())
		})
	})

	Context("filter group entry", func() {
		var (
			group1, group2 *types.Entry
			err            error
		)

		It("create file with tags should be succeed", func() {
			group1, err = types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-new-group-1",
				Kind: types.GroupKind,
			})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, group1)).Should(BeNil())
			Expect(sqlite.UpdateEntryProperties(context.TODO(), namespace, types.PropertyTypeGroupAttr, group1.ID, types.GroupProperties{
				Source: "rss",
				RSS:    &types.GroupRSS{SiteURL: "https://test.com/feed.xml"},
			})).Should(BeNil())

			group2, err = types.InitNewEntry(rootEn, types.EntryAttr{
				Name:            "test-new-group-2",
				Kind:            types.SmartGroupKind,
				GroupProperties: &types.GroupProperties{Filter: &types.Filter{CELPattern: ""}},
			})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, group2)).Should(BeNil())
			Expect(sqlite.UpdateEntryProperties(context.TODO(), namespace, types.PropertyTypeGroupAttr, group2.ID, types.GroupProperties{
				Filter: &types.Filter{CELPattern: ""},
			})).Should(BeNil())
		})
		It("filter rss group be succeed", func() {
			it, err := sqlite.FilterEntries(ctx, namespace, types.Filter{CELPattern: `group.source == "rss"`})
			Expect(err).Should(BeNil())

			hasTag := make(map[int64]struct{})
			for it.HasNext() {
				next, err := it.Next()
				Expect(err).Should(BeNil())
				hasTag[next.ID] = struct{}{}
			}

			_, ok := hasTag[group1.ID]
			Expect(ok).Should(BeTrue())
		})
	})

	Context("filter with pagination", func() {
		var files []*types.Entry

		It("create multiple files should be succeed", func() {
			for i := 0; i < 5; i++ {
				file, err := types.InitNewEntry(rootEn, types.EntryAttr{
					Name: "page-test-file-" + string(rune('a'+i)),
					Kind: types.RawKind,
				})
				Expect(err).Should(BeNil())
				Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, file)).Should(BeNil())
				files = append(files, file)
			}
		})

		It("filter with pagination should return limited results", func() {
			pg := types.NewPagination(1, 2)
			pctx := types.WithPagination(ctx, pg)
			it, err := sqlite.FilterEntries(pctx, namespace, types.Filter{CELPattern: "name.startsWith('page-test-file-')"})
			Expect(err).Should(BeNil())

			count := 0
			for it.HasNext() {
				_, err := it.Next()
				Expect(err).Should(BeNil())
				count++
			}
			Expect(count).To(Equal(2))
		})

		It("filter with second page should return correct offset", func() {
			pg := types.NewPagination(2, 2)
			pctx := types.WithPagination(ctx, pg)
			it, err := sqlite.FilterEntries(pctx, namespace, types.Filter{CELPattern: "name.startsWith('page-test-file-')"})
			Expect(err).Should(BeNil())

			count := 0
			for it.HasNext() {
				_, err := it.Next()
				Expect(err).Should(BeNil())
				count++
			}
			Expect(count).To(Equal(2))
		})

		It("filter without pagination should return all results", func() {
			it, err := sqlite.FilterEntries(ctx, namespace, types.Filter{CELPattern: "name.startsWith('page-test-file-')"})
			Expect(err).Should(BeNil())

			count := 0
			for it.HasNext() {
				_, err := it.Next()
				Expect(err).Should(BeNil())
				count++
			}
			Expect(count).Should(BeNumerically(">=", 5))
		})
	})

	Context("filter workflow job by status", func() {
		var wfJob1, wfJob2, wfJob3 *types.WorkflowJob
		var err error

		It("create workflow and jobs with different statuses should succeed", func() {
			wf := &types.Workflow{
				Id:       "test-workflow",
				Name:     "Test Workflow",
				Enable:   true,
				QueueName: "default",
			}
			err = sqlite.SaveWorkflow(context.TODO(), namespace, wf)
			Expect(err).Should(BeNil())

			wfJob1 = &types.WorkflowJob{
				Id:        "test-wf-job-1",
				Namespace: namespace,
				Workflow:  "test-workflow",
				Status:    "running",
			}
			err = sqlite.SaveWorkflowJob(context.TODO(), namespace, wfJob1)
			Expect(err).Should(BeNil())

			wfJob2 = &types.WorkflowJob{
				Id:        "test-wf-job-2",
				Namespace: namespace,
				Workflow:  "test-workflow",
				Status:    "succeed",
			}
			err = sqlite.SaveWorkflowJob(context.TODO(), namespace, wfJob2)
			Expect(err).Should(BeNil())

			wfJob3 = &types.WorkflowJob{
				Id:        "test-wf-job-3",
				Namespace: namespace,
				Workflow:  "test-workflow",
				Status:    "failed",
			}
			err = sqlite.SaveWorkflowJob(context.TODO(), namespace, wfJob3)
			Expect(err).Should(BeNil())
		})

		It("filter job by single status should return matching jobs", func() {
			jobs, err := sqlite.ListWorkflowJobs(ctx, namespace, types.JobFilter{
				WorkFlowID: "test-workflow",
				Status:     "running",
			})
			Expect(err).Should(BeNil())
			Expect(len(jobs)).To(Equal(1))
			Expect(jobs[0].Id).To(Equal("test-wf-job-1"))
		})

		It("filter jobs by multiple statuses should return matching jobs", func() {
			jobs, err := sqlite.ListWorkflowJobs(ctx, namespace, types.JobFilter{
				WorkFlowID: "test-workflow",
				Statuses:   []string{"running", "succeed"},
			})
			Expect(err).Should(BeNil())
			Expect(len(jobs)).To(Equal(2))
		})

		It("filter jobs with empty statuses should return all jobs", func() {
			jobs, err := sqlite.ListWorkflowJobs(ctx, namespace, types.JobFilter{
				WorkFlowID: "test-workflow",
			})
			Expect(err).Should(BeNil())
			Expect(len(jobs)).To(Equal(3))
		})
	})
})
