// Package events is Maven's event backbone: typed lifecycle taps and turn-completion signals.
// Publishers must finish quickly (no gateway or bus re-entry). Subscribers run in goroutines.
package events

import (
	"context"
	"time"
)

// Event carries a type label, optional string attrs, and an optional typed payload.
type Event struct {
	Type    string
	Attrs   map[string]string
	Payload any
}

// TurnCompleted is the payload for EventTurnCompleted.
type TurnCompleted struct {
	UserMsg      string
	AssistantMsg string
	SessionID    string
	Channel      string
	ChatID       string
	At           time.Time
}

// Event type constants.
const (
	EventBusPublishFailure      = "bus.publish_failure"
	EventBusClosed              = "bus.closed"
	EventOutboundDeliveryFailed = "outbound.delivery_failed"
	EventStreamFailed           = "stream.failed"
	EventTurnCompleted          = "turn.completed"
)

// EventPublisher receives events from subsystems (e.g. kernel/bus).
type EventPublisher interface {
	Publish(context.Context, Event)
}

// NoOp drops all events.
type NoOp struct{}

func (NoOp) Publish(context.Context, Event) {}

func OrPublisher(p EventPublisher) EventPublisher {
	if p == nil {
		return NoOp{}
	}
	return p
}
