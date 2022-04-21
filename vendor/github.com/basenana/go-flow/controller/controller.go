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

package controller

import (
	"context"
	"fmt"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/go-flow/log"
	"github.com/basenana/go-flow/storage"
	"reflect"
)

type FlowController struct {
	flows   map[flow.FID]*runner
	storage storage.Interface
	logger  log.Logger
}

func (c FlowController) Register(f flow.Flow) error {
	if reflect.TypeOf(f).Kind() != reflect.Ptr {
		return fmt.Errorf("flow %v obj not ptr", f)
	}
	flow.FlowTypes[f.Type()] = reflect.TypeOf(f)
	return nil
}

func (c *FlowController) TriggerFlow(ctx context.Context, flowId flow.FID) error {
	f, err := c.storage.GetFlow(flowId)
	if err != nil {
		return err
	}

	r := &runner{
		Flow:    f,
		stopCh:  make(chan struct{}),
		storage: c.storage,
		logger:  c.logger.With(fmt.Sprintf("flow.%s", f.ID())),
	}
	c.flows[f.ID()] = r

	c.logger.Infof("trigger flow %s", flowId)
	return r.Start(&flow.Context{
		Context: ctx,
		Logger:  c.logger.With(fmt.Sprintf("flow.%s", flowId)),
		FlowId:  flowId,
	})
}

func (c *FlowController) PauseFlow(flowId flow.FID) error {
	r, ok := c.flows[flowId]
	if !ok {
		return fmt.Errorf("flow %s not found", flowId)
	}
	if r.GetStatus() == flow.RunningStatus {
		c.logger.Infof("pause flow %s", flowId)
		return r.Pause(fsm.Event{
			Type:   flow.ExecutePauseEvent,
			Status: r.GetStatus(),
			Obj:    r.Flow,
		})
	}
	return fmt.Errorf("flow current is %s, can not pause", r.GetStatus())
}

func (c *FlowController) CancelFlow(flowId flow.FID) error {
	r, ok := c.flows[flowId]
	if !ok {
		return fmt.Errorf("flow %s not found", flowId)
	}
	switch r.GetStatus() {
	case flow.RunningStatus, flow.PausedStatus:
		c.logger.Infof("cancel flow %s", flowId)
		return r.Cancel(fsm.Event{
			Type:    flow.ExecuteCancelEvent,
			Status:  r.GetStatus(),
			Message: "canceled",
			Obj:     r.Flow,
		})
	default:
		return fmt.Errorf("flow current is %s, can not cancel", r.GetStatus())
	}
}

func (c *FlowController) ResumeFlow(flowId flow.FID) error {
	r, ok := c.flows[flowId]
	if !ok {
		return fmt.Errorf("flow %s not found", flowId)
	}
	switch r.GetStatus() {
	case flow.PausedStatus:
		c.logger.Infof("resume flow %s", flowId)
		return r.Resume(fsm.Event{
			Type:   flow.ExecuteResumeEvent,
			Status: r.GetStatus(),
			Obj:    r.Flow,
		})
	default:
		return fmt.Errorf("flow current is %s, can not resume", r.GetStatus())
	}
}

func (c *FlowController) CleanFlow(flowId flow.FID) error {
	r, ok := c.flows[flowId]
	if !ok {
		return fmt.Errorf("flow %s not found", flowId)
	}
	switch r.GetStatus() {
	case flow.SucceedStatus, flow.FailedStatus, flow.CanceledStatus:
		c.logger.Infof("clean flow %s", flowId)
		delete(c.flows, flowId)
	default:
		return fmt.Errorf("flow current is %s, can not clean", r.GetStatus())
	}
	return nil
}

func NewFlowController(opt Option) (*FlowController, error) {
	return &FlowController{
		flows:   make(map[flow.FID]*runner),
		storage: opt.Storage,
		logger:  log.NewLogger("go-flow"),
	}, nil
}
