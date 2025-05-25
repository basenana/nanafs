package bus

import "sync"

var (
	sb *Bus
)

func init() {
	sb = &Bus{
		listeners: map[string]*Listener{},
		exchange:  newExchange(),
	}
}

type Bus struct {
	listeners map[string]*Listener
	exchange  *exchange
	mux       sync.RWMutex
}

func (b *Bus) Subscribe(l *Listener) {
	b.mux.Lock()
	b.listeners[l.id] = l
	b.exchange.add(l.topic, l.id)
	b.mux.Unlock()
}

func (b *Bus) Unsubscribe(lid string) {
	b.mux.Lock()
	b.unsubscribeWithLock(lid)
	b.mux.Unlock()
}

func (b *Bus) unsubscribeWithLock(lid string) {
	delete(b.listeners, lid)
	b.exchange.remove(lid)
}

func (b *Bus) Publish(topic string, args ...interface{}) {
	var needDo []*Listener
	b.mux.Lock()
	lIDs := b.exchange.route(topic)
	for i, lID := range lIDs {
		needDo = append(needDo, b.listeners[lID])
		if needDo[i].once {
			b.unsubscribeWithLock(lID)
		}
	}
	b.mux.Unlock()

	for i := range needDo {
		l := needDo[i]
		go func() {
			l.call(args...)
		}()
	}
}

func Subscribe(topic string, fn interface{}) string {
	l := NewListener(topic, fn, false, false)
	sb.Subscribe(l)
	return l.id
}

func SubscribeOnce(topic string, fn interface{}) string {
	l := NewListener(topic, fn, false, true)

	sb.Subscribe(l)
	return l.id
}

func SubscribeWithBlock(topic string, fn interface{}) string {
	l := NewListener(topic, fn, true, false)

	sb.Subscribe(l)
	return l.id
}

func Unsubscribe(lid string) {
	sb.Unsubscribe(lid)
}

func Publish(topic string, args ...interface{}) {
	sb.Publish(topic, args...)
}
