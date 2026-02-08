package workflow

import (
	"fmt"
	"strings"

	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

func NamespaceDefaultsWorkflow(namespace string) []*types.Workflow {
	return []*types.Workflow{
		{
			Id:        BuildInWorkflowID(namespace, "rss"),
			Name:      "RSS Collect",
			Namespace: namespace,
			Enable:    true,
			QueueName: types.WorkflowQueueFile,
			Trigger: types.WorkflowTrigger{
				RSS:      &types.WorkflowRssTrigger{},
				Interval: utils.ToPtr(30),
			},
			Nodes: []types.WorkflowNode{
				{
					Name: "fetch_rss",
					Type: "rss",
					Params: map[string]string{
						"file_type":    "webarchive",
						"timeout":      "120",
						"clutter_free": "true",
					},
					Input: map[string]interface{}{
						"parent_uri": "$.trigger.parent_uri",
						"feed":       "$.trigger.feed",
					},
					Next: "process_articles",
				},
				{
					Name: "process_articles",
					Type: "docloader",
					Matrix: &types.WorkflowNodeMatrix{
						Data: map[string]any{
							"file_path":  "$.fetch_rss.articles.*.file_path",
							"title":      "$.fetch_rss.articles.*.title",
							"url":        "$.fetch_rss.articles.*.url",
							"site_url":   "$.fetch_rss.articles.*.site_url",
							"site_name":  "$.fetch_rss.articles.*.site_name",
							"updated_at": "$.fetch_rss.articles.*.updated_at",
						},
					},
					Input: map[string]interface{}{
						"file_path":  "$.matrix.file_path",
						"title":      "$.matrix.title",
						"url":        "$.matrix.url",
						"site_url":   "$.matrix.site_url",
						"site_name":  "$.matrix.site_name",
						"updated_at": "$.matrix.updated_at",
					},
					Next: "save_to_nanafs",
				},
				{
					Name: "save_to_nanafs",
					Type: "save",
					Matrix: &types.WorkflowNodeMatrix{
						Data: map[string]any{
							"file_path": "$.process_articles.matrix_results.*.file_path",
							"document":  "$.process_articles.matrix_results.*.document",
						},
					},
					Input: map[string]interface{}{
						"parent_uri": "$.trigger.parent_uri",
						"file_path":  "$.matrix.file_path",
						"document":   "$.matrix.document",
					},
				},
			},
		},
		{
			Id:        BuildInWorkflowID(namespace, "docloader"),
			Name:      "Document Load",
			Namespace: namespace,
			Enable:    true,
			QueueName: types.WorkflowQueueFile,
			Trigger: types.WorkflowTrigger{
				LocalFileWatch: &types.WorkflowLocalFileWatch{
					Event:     events.ActionTypeCreate,
					FileTypes: "pdf,md,markdown,html,webarchive",
				},
			},
			Nodes: []types.WorkflowNode{
				{
					Name: "process_articles",
					Type: "docloader",
					Input: map[string]interface{}{
						"file_path": "$.trigger.file_path",
					},
					Next: "save_to_nanafs",
				},
				{
					Name: "save_to_nanafs",
					Type: "update",
					Input: map[string]interface{}{
						"entry_uri": "$.trigger.entry_uri",
						"document":  "$.process_articles.document",
					},
				},
			},
		},
	}
}

const buildInWorkflowPrefix = "build-in-"

func BuildInWorkflowID(namespace, wfName string) string {
	data := map[string]string{"namespace": namespace, "wf_name": wfName}
	return fmt.Sprintf("%s%s-%s-%s", buildInWorkflowPrefix, strings.ToLower(namespace), strings.ToLower(wfName), utils.ComputeStructHash(data, nil))
}
