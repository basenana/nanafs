package dentry

import (
	"encoding/base64"
	"github.com/basenana/nanafs/pkg/types"
)

func AddInternalAnnotation(obj *types.Object, key, value string, encode bool) {
	obj.ExtendData.Annotation.Add(&types.AnnotationItem{
		Key:        key,
		Content:    value,
		IsInternal: true,
		Encode:     encode,
	})
}

func GetInternalAnnotation(obj *types.Object, key string) *types.AnnotationItem {
	if obj.ExtendData.Annotation != nil {
		item := obj.ExtendData.Annotation.Get(key, true)
		return item
	}
	return nil
}

func DeleteAnnotation(obj *types.Object, key string) {
	obj.ExtendData.Annotation.Remove(key)
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
