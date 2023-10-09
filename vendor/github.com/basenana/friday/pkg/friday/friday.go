/*
 Copyright 2023 Friday Author.

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
	"github.com/basenana/friday/pkg/embedding"
	"github.com/basenana/friday/pkg/llm"
	"github.com/basenana/friday/pkg/spliter"
	"github.com/basenana/friday/pkg/utils/logger"
	"github.com/basenana/friday/pkg/vectorstore"
)

const defaultTopK = 6

var (
	Fri *Friday
)

type Friday struct {
	Log logger.Logger

	LLM       llm.LLM
	Embedding embedding.Embedding
	Vector    vectorstore.VectorStore
	Spliter   spliter.Spliter
}
