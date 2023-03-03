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

package dentry

import (
	"encoding/base64"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	InternalAnnPrefix = "internal.basenana.org"
)

func AddInternalAnnotation(obj *types.Object, key, value string, encode bool) {
	obj.Annotation.Add(&types.AnnotationItem{
		Key:     fmt.Sprintf("%s/%s", InternalAnnPrefix, key),
		Content: value,
		Encode:  encode,
	})
}

func GetInternalAnnotation(obj *types.Object, key string) *types.AnnotationItem {
	if obj.Annotation != nil {
		item := obj.Annotation.Get(fmt.Sprintf("%s/%s", InternalAnnPrefix, key))
		return item
	}
	return nil
}

func DeleteAnnotation(obj *types.Object, key string) {
	obj.Annotation.Remove(key)
}

func RawData2AnnotationContent(raw []byte) string {
	return base64.StdEncoding.EncodeToString(raw)
}

func AnnotationContent2RawData(ann *types.AnnotationItem) ([]byte, error) {
	if !ann.Encode {
		return []byte{}, nil
	}
	return base64.StdEncoding.DecodeString(ann.Content)
}
