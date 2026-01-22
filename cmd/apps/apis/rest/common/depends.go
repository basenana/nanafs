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

package common

import (
	"os"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/dispatch"
	"github.com/basenana/nanafs/pkg/indexer"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/workflow"
)

type Depends struct {
	Meta       metastore.Meta
	Indexer    indexer.Indexer
	Workflow   workflow.Workflow
	Dispatcher *dispatch.Dispatcher
	Notify     *notify.Notify
	Config     config.Config
	Core       core.Core
}

func InitDepends(cfg config.Config, meta metastore.Meta) (*Depends, error) {
	var (
		bCfg = cfg.GetBootstrapConfig()
		err  error
	)

	dep := &Depends{
		Meta:   meta,
		Notify: notify.NewNotify(meta),
		Config: cfg,
	}

	dep.Core, err = core.New(meta, bCfg)
	if err != nil {
		return nil, err
	}

	var tokenizer indexer.Tokenizer
	if dictPath := os.Getenv("STATIC_JIEBA_DICT"); dictPath != "" {
		tokenizer, err = indexer.NewJiebaTokenizer(dictPath)
		if err != nil {
			return nil, err
		}
	} else {
		tokenizer = indexer.NewSpaceTokenizer()
	}
	dep.Indexer, err = indexer.NewMetaDB(meta, tokenizer)
	if err != nil {
		return nil, err
	}

	dep.Workflow, err = workflow.New(dep.Core, dep.Notify, meta, dep.Indexer, bCfg.Workflow)
	if err != nil {
		return nil, err
	}

	dep.Dispatcher, err = dispatch.Init(dep.Core, dep.Notify, meta, meta, dep.Workflow)
	if err != nil {
		return nil, err
	}
	return dep, nil
}
