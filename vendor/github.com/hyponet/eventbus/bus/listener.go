package bus

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

var gid uint64

func init() {
	gid = uint64(rand.Uint32())
}

type Listener struct {
	id    string
	topic string
	fn    reflect.Value
	block bool
	once  bool
	mux   sync.Mutex
}

func (l *Listener) call(args ...interface{}) {
	if l.block {
		l.mux.Lock()
		defer l.mux.Unlock()
	}
	l.fn.Call(l.parseArgs(args...))
}

func (l *Listener) parseArgs(inArgs ...interface{}) (args []reflect.Value) {
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

func NewListener(topic string, fn interface{}, block, once bool) *Listener {
	if fn == nil {
		panic("handler must be function")
	}

	handler := reflect.ValueOf(fn)
	if handler.Type().Kind() != reflect.Func {
		panic("handler not a function")
	}

	return &Listener{
		id:    fmt.Sprintf("%d.%d", time.Now().Nanosecond(), atomic.AddUint64(&gid, 1)),
		topic: topic,
		fn:    handler,
		block: block,
		once:  once,
	}
}
