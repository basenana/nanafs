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

package jobrun

import (
	"github.com/basenana/nanafs/pkg/types"
	"sync"
)

const (
	maxPreNamespace = 100
)

type GroupJobQueue struct {
	groups map[string]*NamespacedJobQueue
}

func newQueue() *GroupJobQueue {
	return &GroupJobQueue{
		groups: map[string]*NamespacedJobQueue{
			types.WorkflowQueueFile: newNamespacedQueue(),
			types.WorkflowQueuePipe: newNamespacedQueue(),
		},
	}
}

func (g *GroupJobQueue) Pop(group string) string {
	grpQ, ok := g.groups[group]
	if !ok {
		return ""
	}
	return grpQ.Pop()
}

func (g *GroupJobQueue) Put(ns, group, jid string) {
	grpQ, ok := g.groups[group]
	if !ok {
		return
	}
	grpQ.Put(ns, jid)
}

func (g *GroupJobQueue) Signal(group string) chan struct{} {
	grpQ, ok := g.groups[group]
	if !ok {
		return nil
	}
	return grpQ.Signal()
}

type NamespacedJobQueue struct {
	namespaces       map[string]*JobQueue
	namespaceList    []string
	namespaceInQueue map[string]struct{}
	signalCh         chan struct{}
	mux              sync.Mutex
}

func (n *NamespacedJobQueue) Pop() string {
	if len(n.namespaceList) == 0 {
		return ""
	}

	n.mux.Lock()

	ns := n.namespaceList[0]
	n.namespaceList = n.namespaceList[1:]
	delete(n.namespaceInQueue, ns)

	q := n.namespaces[ns]

	n.mux.Unlock()

	if q == nil {
		return ""
	}

	return q.Pop()
}

func (n *NamespacedJobQueue) Put(ns, jid string) {
	n.mux.Lock()

	q := n.namespaces[ns]
	if q == nil {
		q = newJobQueue()
		n.namespaces[ns] = q
	}

	q.Put(jid)

	if _, ok := n.namespaceInQueue[ns]; ok {
		n.mux.Unlock()
		return
	}

	n.namespaceList = append(n.namespaceList, ns)
	n.namespaceInQueue[ns] = struct{}{}

	n.mux.Unlock()

	select {
	case n.signalCh <- struct{}{}:
	default:
	}
}

func (n *NamespacedJobQueue) Signal() chan struct{} {
	return n.signalCh
}

func newNamespacedQueue() *NamespacedJobQueue {
	return &NamespacedJobQueue{
		namespaces:       make(map[string]*JobQueue),
		namespaceList:    make([]string, 1024),
		namespaceInQueue: make(map[string]struct{}),
		signalCh:         make(chan struct{}, maxPreNamespace),
	}
}

type JobQueue struct {
	queue   []string
	inQueue map[string]struct{}
	mux     sync.Mutex
}

func newJobQueue() *JobQueue {
	return &JobQueue{
		queue:   make([]string, 0, maxPreNamespace),
		inQueue: make(map[string]struct{}),
	}
}

func (j *JobQueue) Pop() string {
	j.mux.Lock()
	if len(j.queue) == 0 {
		j.mux.Unlock()
		return ""
	}

	pop := j.queue[0]
	j.queue = j.queue[1:]
	delete(j.inQueue, pop)
	j.mux.Unlock()
	return pop
}

func (j *JobQueue) Put(jid string) {
	if len(j.queue) > maxPreNamespace {
		return
	}

	j.mux.Lock()
	if _, ok := j.inQueue[jid]; ok {
		j.mux.Unlock()
		return
	}
	j.queue = append(j.queue, jid)
	j.inQueue[jid] = struct{}{}
	j.mux.Unlock()
}
