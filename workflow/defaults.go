package workflow

import (
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

func NamespaceDefaultsWorkflow(namespace string) []*types.Workflow {
	return []*types.Workflow{
		{
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
						"feed": "$.trigger.feed",
					},
					Next: "process_articles",
				},
				{
					Name: "process_articles",
					Type: "docloader",
					Matrix: &types.WorkflowNodeMatrix{
						Data: map[string]any{
							"file_path": "$.fetch_rss.articles.*.file_path",
						},
					},
					Input: map[string]interface{}{
						"file_path": "$.matrix.file_path",
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
