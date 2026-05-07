package bus

import (
	"context"
	"sync"

	mavenlog "github.com/ageneralai/maven/internal/log"
)

type MessageBus struct {
	Inbound  chan InboundMessage
	Outbound chan OutboundMessage
	log      mavenlog.PrintLogger

	mu   sync.RWMutex
	subs map[string][]func(OutboundMessage)
}

func NewMessageBus(bufSize int, log mavenlog.PrintLogger) *MessageBus {
	if bufSize <= 0 {
		bufSize = 100
	}
	return &MessageBus{
		Inbound:  make(chan InboundMessage, bufSize),
		Outbound: make(chan OutboundMessage, bufSize),
		log:      log,
		subs:     make(map[string][]func(OutboundMessage)),
	}
}

func (b *MessageBus) SubscribeOutbound(channel string, fn func(OutboundMessage)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[channel] = append(b.subs[channel], fn)
}

func (b *MessageBus) DispatchOutbound(ctx context.Context) {
	for {
		select {
		case msg := <-b.Outbound:
			b.mu.RLock()
			cbs := b.subs[msg.Channel]
			b.mu.RUnlock()
			for _, cb := range cbs {
				cb(msg)
			}
			if len(cbs) == 0 {
				b.log.Printf("[bus] no subscriber for channel %q, dropping message", msg.Channel)
			}
		case <-ctx.Done():
			return
		}
	}
}
