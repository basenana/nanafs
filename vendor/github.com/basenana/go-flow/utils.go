/*
   Copyright 2024 Go-Flow Authors

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
	"runtime/debug"
)

func doRecover() error {
	if panicErr := recover(); panicErr != nil {
		debug.PrintStack()
		return fmt.Errorf("panic: %v", panicErr)
	}
	return nil
}

type StringSet map[string]struct{}

func (s StringSet) Insert(strList ...string) {
	for _, str := range strList {
		s[str] = struct{}{}
	}
}
func (s StringSet) Has(str string) bool {
	_, ok := s[str]
	return ok
}

func (s StringSet) Del(str string) {
	if _, ok := s[str]; ok {
		delete(s, str)
	}
}

func (s StringSet) List() (result []string) {
	for k := range s {
		result = append(result, k)
	}
	return
}

func (s StringSet) Len() int {
	return len(s)
}

func NewStringSet(strList ...string) StringSet {
	ss := map[string]struct{}{}
	for _, str := range strList {
		ss[str] = struct{}{}
	}
	return ss
}
