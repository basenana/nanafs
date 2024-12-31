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
	"fmt"
	"sync"
)

type statusEvent struct {
	Type    string
	Status  string
	Message string
}

type fsmEventHandler func(evt statusEvent) error

type edge struct {
	from string
	to   string
	when string
	do   fsmEventHandler
	next *edge
}

type edgeBuilder struct {
	from []string
	to   string
	when string
	do   fsmEventHandler
}

type FSM struct {
	status string
	graph  map[string]*edge

	crtBuilder *edgeBuilder
	mux        sync.Mutex
}

func (m *FSM) From(statues []string) *FSM {
	m.buildWarp(func(builder *edgeBuilder) {
		builder.from = statues
	})
	return m
}

func (m *FSM) To(status string) *FSM {
	m.buildWarp(func(builder *edgeBuilder) {
		builder.to = status
	})
	return m
}

func (m *FSM) When(event string) *FSM {
	m.buildWarp(func(builder *edgeBuilder) {
		builder.when = event
	})
	return m
}

func (m *FSM) Do(handler fsmEventHandler) *FSM {
	m.buildWarp(func(builder *edgeBuilder) {
		builder.do = handler
	})
	return m
}

func (m *FSM) Event(event statusEvent) (err error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	head := m.graph[event.Type]
	if head == nil {
		return nil
	}

	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("panic: %s", panicErr)
		}
	}()

	for head != nil {
		if m.status == head.from {
			m.status = head.to
			event.Status = m.status
			if head.do != nil {
				if handleErr := head.do(event); handleErr != nil {
					return handleErr
				}
			}
			return nil
		}
		head = head.next
	}
	err = fmt.Errorf("get event %s and current status is %s, no change path matched", event.Type, m.status)
	return err
}

func (m *FSM) buildWarp(f func(builder *edgeBuilder)) {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.crtBuilder == nil {
		m.crtBuilder = &edgeBuilder{}
	}
	f(m.crtBuilder)

	if len(m.crtBuilder.from) == 0 {
		return
	}

	if m.crtBuilder.to == "" {
		return
	}

	if m.crtBuilder.when == "" {
		return
	}

	if m.crtBuilder.do == nil {
		return
	}

	builder := m.crtBuilder
	m.crtBuilder = nil

	head := m.graph[builder.when]
	for _, from := range builder.from {
		newEdge := &edge{
			from: from,
			to:   builder.to,
			when: builder.when,
			do:   builder.do,
			next: head,
		}
		head = newEdge
		m.graph[builder.when] = newEdge
	}
}

func NewFSM(status string) *FSM {
	f := &FSM{status: status, graph: make(map[string]*edge)}
	return f
}
