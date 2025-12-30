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
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(engine *gin.Engine, s *ServicesV1) {
	v1 := engine.Group("/api/v1")
	{
		// Entries
		entries := v1.Group("/entries")
		{
			entries.GET("/:uri", s.GetEntryDetail)
			entries.POST("", s.CreateEntry)
			entries.POST("/search", s.FilterEntry)
			entries.PUT("/:uri", s.UpdateEntry)
			entries.DELETE("/:uri", s.DeleteEntry)
			entries.POST("/batch-delete", s.DeleteEntries)
			entries.PUT("/:uri/parent", s.ChangeParent)
			entries.PUT("/:uri/properties", s.UpdateProperty)
			entries.PUT("/:uri/document", s.UpdateDocumentProperty)
		}

		// Groups
		groups := v1.Group("/groups")
		{
			groups.GET("/:uri/children", s.ListGroupChildren)
		}

		// Files
		files := v1.Group("/files")
		{
			files.GET("/:entry/content", s.ReadFile)
			files.POST("/:entry/content", s.WriteFile)
		}

		// Tree
		v1.GET("/tree", s.GroupTree)

		// Messages
		v1.GET("/messages", s.ListMessages)
		v1.POST("/messages/read", s.ReadMessages)

		// Workflows
		workflows := v1.Group("/workflows")
		{
			workflows.GET("", s.ListWorkflows)
			workflows.GET("/:id/jobs", s.ListWorkflowJobs)
			workflows.POST("/:id/trigger", s.TriggerWorkflow)
		}
	}
}
