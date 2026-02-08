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

// @title NanaFS API
// @version 1.0
// @description NanaFS is a Reference Filing System API
// @host localhost:7081
// @BasePath /api/v1
func RegisterRoutes(engine *gin.Engine, s *ServicesV1) {
	v1 := engine.Group("/api/v1")
	{

		/*
			Special Case Explanation:
			Due to uri its complexity, the Path for entry/group/file does not fully comply with RESTful API specifications.
			This is a special case; other APIs still need to conform to RESTful API specifications.
		*/

		// Entries - support body with uri or id
		entries := v1.Group("/entries")
		{
			entries.POST("", s.CreateEntry)
			entries.POST("/filter", s.FilterEntry)
			entries.POST("/search", s.SearchEntry)
			entries.POST("/batch-delete", s.DeleteEntries)
			entries.POST("/delete", s.DeleteEntry)

			entries.POST("/details", s.EntryDetails)
			entries.POST("/property", s.EntryProperty)
			entries.POST("/friday", s.GetFridayProperty)

			entries.PUT("", s.UpdateEntry)
			entries.PUT("/parent", s.ChangeParent)
			entries.PUT("/property", s.UpdateProperty)
			entries.PUT("/document", s.UpdateDocumentProperty)
		}

		// Groups - support body with uri or id
		groups := v1.Group("/groups")
		{
			groups.POST("/children", s.ListGroupChildren)
			groups.GET("/tree", s.GroupTree)
			groups.POST("/configs", s.GetGroupConfig)
			groups.POST("/configs/update", s.SetGroupConfig)
		}

		// Files - support body with uri or id
		files := v1.Group("/files")
		{
			files.POST("/content", s.ReadFile)
			files.POST("/content/write", s.WriteFile)
		}

		// Messages
		v1.GET("/messages", s.ListMessages)
		v1.POST("/messages/read", s.ReadMessages)

		// Workflows
		workflows := v1.Group("/workflows")
		{
			workflows.GET("", s.ListWorkflows)
			workflows.POST("", s.CreateWorkflow)
			workflows.GET("/:id", s.GetWorkflow)
			workflows.PUT("/:id", s.UpdateWorkflow)
			workflows.DELETE("/:id", s.DeleteWorkflow)
			workflows.GET("/:id/jobs", s.ListWorkflowJobs)
			workflows.GET("/:id/jobs/:jobId", s.GetJob)
			workflows.POST("/:id/jobs/:jobId/pause", s.PauseJob)
			workflows.POST("/:id/jobs/:jobId/resume", s.ResumeJob)
			workflows.POST("/:id/jobs/:jobId/cancel", s.CancelJob)
			workflows.POST("/:id/trigger", s.TriggerWorkflow)
			workflows.GET("/plugins", s.ListPlugins)
		}

		// Configs - Namespace-level configuration center
		configs := v1.Group("/configs")
		{
			configs.GET("/:group/:name", s.GetConfig)
			configs.PUT("/:group/:name", s.SetConfig)
			configs.GET("/:group", s.ListConfig)
			configs.DELETE("/:group/:name", s.DeleteConfig)
		}
	}
}
