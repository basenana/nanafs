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
	"runtime/trace"
)

func TraceTask(ctx context.Context, taskName string) (context.Context, func()) {
	ctx, t := trace.NewTask(ctx, taskName)
	return ctx, func() {
		t.End()
	}
}

func TraceRegion(ctx context.Context, message string, args ...interface{}) func() {
	t := trace.StartRegion(ctx, fmt.Sprintf(message, args...))
	return func() {
		t.End()
	}
}
