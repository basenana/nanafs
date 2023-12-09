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

package friday

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/pkg/friday"
)

type question struct {
	Question string `json:"question"`
}

func Question(gCtx *gin.Context) {
	var q question
	if err := gCtx.ShouldBindJSON(&q); err != nil {
		gCtx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid question"})
		return
	}
	answer, usage, err := friday.Question(gCtx, q.Question)
	if err != nil {
		gCtx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	res := map[string]any{
		"status": "ok",
		"answer": answer,
		"time":   time.Now().Format(time.RFC3339),
	}
	for k, v := range usage {
		res[k] = v
	}

	gCtx.JSON(200, res)
}
