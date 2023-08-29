/*
   Copyright 2022 Go-Flow Authors

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

package flow

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"strings"
	"sync"
)

type Controller struct {
	runners map[string]Runner
	mux     sync.Mutex
	storage Storage
	logger  *zap.SugaredLogger
}

func (c *Controller) TriggerFlow(ctx context.Context, flowId string) error {
	f, err := c.storage.GetFlow(ctx, flowId)
	if err != nil {
		return err
	}
	r := NewRunner(f, c.storage)

	c.mux.Lock()
	c.runners[f.ID] = r
	c.mux.Unlock()

	c.logger.Infof("trigger flow %s", flowId)
	go c.startFlowRunner(ctx, f)
	return nil
}

func (c *Controller) startFlowRunner(ctx context.Context, flow *Flow) {
	c.mux.Lock()
	r, ok := c.runners[flow.ID]
	c.mux.Unlock()
	if !ok {
		c.logger.Errorf("start runner failed, err: runner %s not found", flow.ID)
		return
	}

	defer func() {
		c.mux.Lock()
		delete(c.runners, flow.ID)
		c.mux.Unlock()
	}()

	err := r.Start(ctx)
	if err != nil {
		c.logger.Errorf("start runner failed, err: %s", err)
	}
}

func (c *Controller) PauseFlow(flowId string) error {
	r, ok := c.runners[flowId]
	if !ok {
		return fmt.Errorf("flow %s not found", flowId)
	}
	c.logger.Infof("pause flow %s", flowId)
	return r.Pause()
}

func (c *Controller) CancelFlow(flowId string) error {
	r, ok := c.runners[flowId]
	if !ok {
		return fmt.Errorf("flow %s not found", flowId)
	}
	return r.Cancel()
}

func (c *Controller) ResumeFlow(flowId string) error {
	r, ok := c.runners[flowId]
	if !ok {
		return fmt.Errorf("flow %s not found", flowId)
	}
	return r.Resume()
}

func (c *Controller) Shutdown() error {
	c.mux.Lock()
	defer c.mux.Unlock()

	failedFlows := make([]string, 0)
	for fId, r := range c.runners {
		if err := r.Cancel(); err != nil {
			failedFlows = append(failedFlows, fId)
		}
	}
	if len(failedFlows) > 0 {
		return fmt.Errorf("cancel flows failed: %s", strings.Join(failedFlows, ","))
	}
	return nil
}

func NewFlowController(storage Storage) *Controller {
	return &Controller{
		runners: make(map[string]Runner),
		storage: storage,
		logger:  logger.NewLogger("flow"),
	}
}
