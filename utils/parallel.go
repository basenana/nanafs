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
)

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

type MaximumParallel struct {
	q chan struct{}
}

func (p *MaximumParallel) Go(f func()) {
	go func() {
		p.q <- struct{}{}
		defer func() {
			select {
			case <-p.q:
			default:
			}
		}()
		f()
	}()
}

func (p *MaximumParallel) BlockedGo(f func()) {
	p.q <- struct{}{}
	go func() {
		defer func() {
			select {
			case <-p.q:
			default:
			}
		}()
		f()
	}()
}

func NewMaximumParallel(num int) *MaximumParallel {
	return &MaximumParallel{q: make(chan struct{}, num)}
}

func Recover() error {
	if panicErr := recover(); panicErr != nil {
		debug.PrintStack()
		sentry.CurrentHub().Recover(panicErr)
		return fmt.Errorf("panic: %v", panicErr)
	}
	return nil
}
