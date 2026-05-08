// Package events is Maven's minimal event backbone: typed lifecycle and diagnostic taps
// with no heavyweight schema. Publishers must finish quickly (no gateway or bus re-entry).
package events

import "context"

// Event is intentionally small — grow fields only when a consumer needs them.
type Event struct {
	Type  string            // Use EventBusPublishFailure, EventBusClosed, etc.
	Attrs map[string]string // Optional labels; omit or nil when empty.
}

// EventPublisher receives events from subsystems (e.g. internal/bus).
// Implementations must be non-blocking in practice (return fast; offload work async if needed).
type EventPublisher interface {
	Publish(context.Context, Event)
}

const (
	EventBusPublishFailure = "bus.publish_failure"
	EventBusClosed         = "bus.closed"
)

// NoOp drops all events. Use when no observer is wired.
type NoOp struct{}

func (NoOp) Publish(context.Context, Event) {}

func OrPublisher(p EventPublisher) EventPublisher {
	if p == nil {
		return NoOp{}
	}
	return p
}
