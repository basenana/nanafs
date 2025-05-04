/*
 Copyright 2024 NanaFS Authors.

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

package fs

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/dialogue"
	"github.com/basenana/nanafs/pkg/dispatch"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/token"
	"github.com/basenana/nanafs/workflow"
)

type Service struct {
	Commander
	Query
}

func NewService(depends *Depends) (*Service, error) {
	var (
		fs  = &Service{}
		err error
	)

	fs.Query, err = newQuery(depends)
	if err != nil {
		return nil, fmt.Errorf("init fs query service error: %w", err)
	}

	fs.Commander, err = newCommander(depends)
	if err != nil {
		return nil, fmt.Errorf("init fs commander error: %w", err)
	}

	return fs, nil
}

type Depends struct {
	Meta         metastore.Meta
	Workflow     workflow.Workflow
	Dispatcher   *dispatch.Dispatcher
	Notify       *notify.Notify
	Document     document.Manager
	Dialogue     dialogue.Manager
	Token        *token.Manager
	ConfigLoader config.Loader
	Core         core.Core
}

func InitDepends(loader config.Loader, meta metastore.Meta, fridayClient friday.Friday) (*Depends, error) {
	bCfg, err := loader.GetBootstrapConfig()
	if err != nil {
		return nil, err
	}

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

	dep.Document, err = document.NewManager(meta, dep.Core, loader, fridayClient)
	if err != nil {
		return nil, err
	}

	dep.Dialogue, err = dialogue.NewManager(meta)
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
	return dep, nil
}
