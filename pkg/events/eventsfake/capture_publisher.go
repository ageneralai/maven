// Package eventsfake holds test doubles for github.com/ageneralai/maven/pkg/events.
package eventsfake

import (
	"context"
	"sync"

	"github.com/ageneralai/maven/pkg/events"
)

// CapturePublisher records every published event (implements EventPublisher).
type CapturePublisher struct {
	mu   sync.Mutex
	evts []events.Event
}

func (c *CapturePublisher) Publish(_ context.Context, e events.Event) {
	c.mu.Lock()
	c.evts = append(c.evts, e)
	c.mu.Unlock()
}

// Snapshot returns a copy of recorded events (safe for asserts after Publish).
func (c *CapturePublisher) Snapshot() []events.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]events.Event, len(c.evts))
	copy(out, c.evts)
	return out
}
