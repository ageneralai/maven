package events

import (
	"context"
	"sync"
)

var (
	defaultMu        sync.RWMutex
	defaultPublisher EventPublisher = NoOp{}
)

// SetDefaultPublisher replaces the process-wide event sink; nil resets to NoOp.
func SetDefaultPublisher(p EventPublisher) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if p == nil {
		defaultPublisher = NoOp{}
		return
	}
	defaultPublisher = p
}

// Publish sends an event to the default publisher (see SetDefaultPublisher).
func Publish(ctx context.Context, e Event) {
	defaultMu.RLock()
	p := defaultPublisher
	defaultMu.RUnlock()
	p.Publish(ctx, e)
}
