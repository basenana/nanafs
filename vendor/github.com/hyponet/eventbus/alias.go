package eventbus

import "github.com/hyponet/eventbus/bus"

var (
	Subscribe          = bus.Subscribe
	SubscribeOnce      = bus.SubscribeOnce
	SubscribeWithBlock = bus.SubscribeWithBlock
	Unsubscribe        = bus.Unsubscribe
	Publish            = bus.Publish
)
