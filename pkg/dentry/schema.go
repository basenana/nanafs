package dentry

import (
	"github.com/basenana/nanafs/pkg/types"
	"reflect"
)

type SchemaRegistry struct {
	schemas     []Schema
	schemaTypes []types.Kind
}

type Schema struct {
	CType   types.Kind
	Version string
	Spec    reflect.Type
}

var Registry = &SchemaRegistry{schemas: []Schema{}, schemaTypes: []types.Kind{}}

func (s *SchemaRegistry) Register(cType types.Kind, version string, spec interface{}) {
	s.schemas = append(s.schemas, Schema{
		CType:   cType,
		Version: version,
		Spec:    reflect.TypeOf(spec),
	})
	s.schemaTypes = append(s.schemaTypes, cType)
}

func (s *SchemaRegistry) GetSchemas() []Schema {
	return s.schemas
}

func (s *SchemaRegistry) GetSchema(cType types.Kind) interface{} {
	for _, c := range s.schemas {
		if c.CType == cType {
			return reflect.New(c.Spec).Interface()
		}
	}
	return nil
}

func (s *SchemaRegistry) GetSchemaTypes() []types.Kind {
	return s.schemaTypes
}

func (s *SchemaRegistry) IsStructuredType(cType types.Kind) bool {
	for _, c := range s.schemaTypes {
		if c == cType {
			return true
		}
	}
	return false
}
