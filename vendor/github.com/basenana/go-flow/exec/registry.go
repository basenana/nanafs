package exec

import (
	"github.com/basenana/go-flow/flow"
	"sync"
)

type OperatorBuilder func(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error)

type Registry struct {
	builders map[string]OperatorBuilder
	mux      sync.Mutex
}

func (r *Registry) Register(opType string, builder OperatorBuilder) error {
	r.mux.Lock()
	defer r.mux.Unlock()
	if _, existed := r.builders[opType]; existed {
		return OperatorIsExisted
	}
	r.builders[opType] = builder
	return nil
}

func (r *Registry) FindBuilder(opType string) (OperatorBuilder, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	builder, ok := r.builders[opType]
	if !ok {
		return nil, OperatorNotFound
	}
	return builder, nil
}

func NewEmptyRegistry() *Registry {
	return &Registry{builders: map[string]OperatorBuilder{}}
}
