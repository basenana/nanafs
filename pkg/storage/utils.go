package storage

import (
	"database/sql"
	"github.com/basenana/nanafs/pkg/types"
	"sync"
)

type Iterator interface {
	HasNext() bool
	Next() *types.Object
}

type iterator struct {
	objects []*types.Object
	mux     sync.Mutex
}

func (i *iterator) HasNext() bool {
	i.mux.Lock()
	defer i.mux.Unlock()
	return len(i.objects) > 0
}

func (i *iterator) Next() *types.Object {
	i.mux.Lock()
	defer i.mux.Unlock()
	obj := i.objects[0]
	i.objects = i.objects[1:]
	return obj
}

func dbError2Error(err error) error {
	switch err {
	case sql.ErrNoRows:
		return types.ErrNotFound

	default:
		return err
	}
}
