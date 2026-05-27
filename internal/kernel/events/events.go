// Package events is Maven's minimal event backbone: typed lifecycle and diagnostic taps
// with no heavyweight schema. Publishers must finish quickly (no gateway or bus re-entry).
package events

import "context"

// Event is intentionally small — grow fields only when a consumer needs them.
type Event struct {
	Type  string            // Use EventBusPublishFailure, EventBusClosed, etc.
	Attrs map[string]string // Optional labels; omit or nil when empty.
}

// EventPublisher receives events from subsystems (e.g. kernel/bus).
// Implementations must be non-blocking in practice (return fast; offload work async if needed).
type EventPublisher interface {
	Publish(context.Context, Event)
}

// Event type constants for bus and channel lifecycle taps.
const (
	// EventBusPublishFailure is emitted when inbound/outbound publish blocks or times out.
	EventBusPublishFailure = "bus.publish_failure"
	// EventBusClosed is emitted when the message bus shuts down.
	EventBusClosed = "bus.closed"
	// EventOutboundDeliveryFailed is emitted when a channel cannot deliver an outbound message.
	EventOutboundDeliveryFailed = "outbound.delivery_failed"
	// EventStreamFailed is emitted when a streaming reply ends with an error.
	EventStreamFailed = "stream.failed"
)

// NoOp drops all events. Use when no observer is wired.
type NoOp struct{}

// Publish implements EventPublisher by discarding events.
func (NoOp) Publish(context.Context, Event) {}

// OrPublisher returns p when non-nil, otherwise NoOp.
func OrPublisher(p EventPublisher) EventPublisher {
	if p == nil {
		return NoOp{}
	}
	return p
}
