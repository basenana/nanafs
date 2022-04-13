package bus

import (
	"fmt"
	"github.com/google/uuid"
	"reflect"
	"sync"
)

type listener struct {
	id    string
	topic string
	fn    reflect.Value
	block bool
	once  bool
	mux   sync.Mutex
}

func (l *listener) call(args ...interface{}) {
	if l.block {
		l.mux.Lock()
		defer l.mux.Unlock()
	}
	l.fn.Call(l.parseArgs(args...))
}

func (l *listener) parseArgs(inArgs ...interface{}) (args []reflect.Value) {
	fn := l.fn.Type()
	args = make([]reflect.Value, len(inArgs))
	for i, inArg := range inArgs {
		if inArg == nil {
			args[i] = reflect.New(fn.In(i)).Elem()
			continue
		}
		args[i] = reflect.ValueOf(inArg)
	}
	return
}

func buildNewListener(topic string, fn interface{}, block, once bool) (*listener, error) {
	if fn == nil {
		return nil, fmt.Errorf("handler must be function")
	}

	handler := reflect.ValueOf(fn)
	if handler.Type().Kind() != reflect.Func {
		return nil, fmt.Errorf("handler not a function")
	}

	return &listener{
		id:    uuid.New().String(),
		topic: topic,
		fn:    handler,
		block: block,
		once:  once,
	}, nil
}
