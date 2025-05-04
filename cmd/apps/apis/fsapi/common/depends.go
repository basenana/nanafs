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
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/dispatch"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/rule"
	"github.com/basenana/nanafs/pkg/token"
	"github.com/basenana/nanafs/workflow"
)

type Depends struct {
	Meta         metastore.Meta
	Workflow     workflow.Workflow
	Dispatcher   *dispatch.Dispatcher
	Notify       *notify.Notify
	Document     document.Manager
	FridayClient friday.Friday
	Token        *token.Manager
	ConfigLoader config.Config
	Core         core.Core
}

func InitDepends(loader config.Config, meta metastore.Meta) (*Depends, error) {
	var (
		bCfg = loader.GetBootstrapConfig()
		err  error
	)

	dep := &Depends{Meta: meta, ConfigLoader: loader}
	dep.Token = token.NewTokenManager(meta, loader)
	if tokenErr := dep.Token.InitBuildinCA(context.Background()); tokenErr != nil {
		return nil, tokenErr
	}

	dep.Notify = notify.NewNotify(meta)

	dep.Core, err = core.New(meta, bCfg)
	if err != nil {
		return nil, err
	}

	dep.FridayClient = friday.NewFridayClient(bCfg.FridayConfig)
	dep.Document, err = document.NewManager(meta, dep.Core, loader, dep.FridayClient)
	if err != nil {
		return nil, err
	}

	dep.Workflow, err = workflow.New(dep.Core, dep.Document, dep.Notify, meta, loader)
	if err != nil {
		return nil, err
	}

	dep.Dispatcher, err = dispatch.Init(dep.Core, dep.Notify, meta)
	if err != nil {
		return nil, err
	}

	rule.InitQuery(meta)
	return dep, nil
}
