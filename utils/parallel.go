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

package utils

import (
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"runtime/debug"
	"sync/atomic"
	"time"
)

type ParallelWorker struct {
	workQ   chan func()
	workMax int32
	workCrt int32
}

func (p *ParallelWorker) Dispatch(ctx context.Context, fn func()) {
	select {
	case <-ctx.Done():
		return
	case p.workQ <- fn:
		return
	default:
		for {
			crt := atomic.LoadInt32(&p.workCrt)
			if crt == p.workMax {
				break
			}

			if atomic.CompareAndSwapInt32(&p.workCrt, crt, crt+1) {
				go p.workRun()
				break
			}
		}
	}

	select {
	case <-ctx.Done():
		return
	case p.workQ <- fn:
		return
	}
}

func (p *ParallelWorker) workRun() {
	leastWait := time.NewTicker(time.Hour)
	defer leastWait.Stop()
	for {
		select {
		case fn := <-p.workQ:
			fn()
		case <-leastWait.C:
			atomic.AddInt32(&p.workCrt, -1)
			return
		}
	}
}

func NewParallelWorker(num int32) *ParallelWorker {
	return &ParallelWorker{workQ: make(chan func()), workMax: num}
}

type ParallelLimiter struct {
	q chan struct{}
}

func (l *ParallelLimiter) Acquire(ctx context.Context) error {
	select {
	case l.q <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *ParallelLimiter) Release() {
	select {
	case <-l.q:
	default:
	}
}

func NewParallelLimiter(ctn int) *ParallelLimiter {
	return &ParallelLimiter{q: make(chan struct{}, ctn)}
}

func Recover() error {
	if panicErr := recover(); panicErr != nil {
		debug.PrintStack()
		sentry.CurrentHub().Recover(panicErr)
		return fmt.Errorf("panic: %v", panicErr)
	}
	return nil
}
