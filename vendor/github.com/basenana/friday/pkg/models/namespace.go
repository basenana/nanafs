/*
 Copyright 2023 Friday Author.

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

package models

import "context"

const (
	NamespaceKey          = "namespace"
	DefaultNamespaceValue = "global" // TODO: using 'public'
	GlobalNamespaceValue  = "global"
)

type Namespace struct {
	name string
}

func NewNamespace(name string) *Namespace {
	return &Namespace{name: name}
}

func (n *Namespace) String() string {
	return n.name
}

func GetNamespace(ctx context.Context) (ns *Namespace) {
	ns = &Namespace{
		name: DefaultNamespaceValue,
	}
	if ctx.Value(NamespaceKey) != nil {
		ns.name = ctx.Value(NamespaceKey).(string)
	}
	return
}

func WithNamespace(ctx context.Context, ns *Namespace) context.Context {
	return context.WithValue(ctx, NamespaceKey, ns.String())
}
