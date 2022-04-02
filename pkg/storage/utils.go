package storage

import (
	"github.com/basenana/nanafs/pkg/dentry"
	"sync"
)

type Iterator interface {
	HasNext() bool
	Next() *dentry.Entry
}

type iterator struct {
	objects []*dentry.Entry
	mux     sync.Mutex
}

func (i *iterator) HasNext() bool {
	i.mux.Lock()
	defer i.mux.Unlock()
	return len(i.objects) > 0
}

func (i *iterator) Next() *dentry.Entry {
	i.mux.Lock()
	defer i.mux.Unlock()
	obj := i.objects[0]
	i.objects = i.objects[1:]
	return obj
}
