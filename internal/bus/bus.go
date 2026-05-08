package bus

import (
	"context"
	"sync"

	mavenlog "github.com/ageneralai/maven/pkg/log"
)

// MessageBus carries pipeline Inbound work and fans out Outbound to at most one
// subscriber per channel name. Outbound has no delivery acknowledgement — see OutboundMessage.
type MessageBus struct {
	Inbound  chan InboundMessage
	Outbound chan OutboundMessage
	log      mavenlog.PrintLogger

	mu   sync.RWMutex
	subs map[string]func(OutboundMessage)
}

func NewMessageBus(bufSize int, log mavenlog.PrintLogger) *MessageBus {
	if bufSize <= 0 {
		bufSize = 100
	}
	return &MessageBus{
		Inbound:  make(chan InboundMessage, bufSize),
		Outbound: make(chan OutboundMessage, bufSize),
		log:      log,
		subs:     make(map[string]func(OutboundMessage)),
	}
}

// SetOutboundSubscriber registers the single outbound handler for channel.
// Passing nil removes the subscriber.
func (b *MessageBus) SetOutboundSubscriber(channel string, fn func(OutboundMessage)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if fn == nil {
		delete(b.subs, channel)
		return
	}
	b.subs[channel] = fn
}

func (b *MessageBus) DispatchOutbound(ctx context.Context) {
	for {
		select {
		case msg := <-b.Outbound:
			b.mu.RLock()
			cb := b.subs[msg.Channel]
			b.mu.RUnlock()
			if cb != nil {
				cb(msg)
			} else {
				b.log.Printf("[bus] no subscriber for channel %q, dropping message", msg.Channel)
			}
		case <-ctx.Done():
			return
		}
	}
}
