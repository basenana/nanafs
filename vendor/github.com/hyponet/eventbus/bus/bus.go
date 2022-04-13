package bus

import "sync"

var (
	evb *eventbus
)

func init() {
	evb = &eventbus{
		listeners: map[string]*listener{},
		exchange:  newExchange(),
	}
}

type eventbus struct {
	listeners map[string]*listener
	exchange  *exchange
	mux       sync.RWMutex
}

func (b *eventbus) subscribe(l *listener) {
	b.mux.Lock()
	b.listeners[l.id] = l
	b.exchange.add(l.topic, l.id)
	b.mux.Unlock()
}

func (b *eventbus) unsubscribe(lID string) {
	b.mux.Lock()
	b.unsubscribeWithLock(lID)
	b.mux.Unlock()
}

func (b *eventbus) unsubscribeWithLock(lID string) {
	delete(b.listeners, lID)
	b.exchange.remove(lID)
}

func (b *eventbus) publish(topic string, args ...interface{}) {
	var needDo []*listener
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

func Subscribe(topic string, fn interface{}) (string, error) {
	l, err := buildNewListener(topic, fn, false, false)
	if err != nil {
		return "", err
	}

	evb.subscribe(l)
	return l.id, nil
}

func SubscribeOnce(topic string, fn interface{}) (string, error) {
	l, err := buildNewListener(topic, fn, false, true)
	if err != nil {
		return "", err
	}

	evb.subscribe(l)
	return l.id, nil
}

func SubscribeWithBlock(topic string, fn interface{}) (string, error) {
	l, err := buildNewListener(topic, fn, true, false)
	if err != nil {
		return "", err
	}

	evb.subscribe(l)
	return l.id, nil
}

func Unsubscribe(lID string) {
	evb.unsubscribe(lID)
}

func Publish(topic string, args ...interface{}) {
	evb.publish(topic, args...)
}
