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

package api

import (
	"context"
	"io"

	"github.com/basenana/plugin/types"
)

type PersistentStore interface {
	Load(ctx context.Context, source, group, key string, data any) error
	Save(ctx context.Context, source, group, key string, data any) error
}

type NanaFS interface {
	CreateGroupIfNotExists(ctx context.Context, parentURI, group string, properties types.Properties) error
	SaveEntry(ctx context.Context, parentURI, name string, properties types.Properties, reader io.ReadCloser) error
	UpdateEntry(ctx context.Context, entryURI string, properties types.Properties) error
	GetEntryProperties(ctx context.Context, entryURI string) (properties *types.Properties, err error)
}
