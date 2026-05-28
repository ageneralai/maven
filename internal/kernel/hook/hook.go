// Package hook defines the PostTurnEvent and PostTurnHandler types shared between
// the pipeline (emitter) and the plugin layer (consumer) without creating an import cycle.
package hook

import (
	"context"
	"time"
)

// PostTurnEvent is emitted by the pipeline after each real user conversation turn completes.
type PostTurnEvent struct {
	UserMsg      string
	AssistantMsg string
	SessionID    string
	Channel      string
	ChatID       string
	At           time.Time
}

// PostTurnHandler is called by the pipeline after each successful user conversation turn.
// The pipeline fires it in a goroutine; implementations must not spawn additional goroutines.
type PostTurnHandler func(ctx context.Context, ev PostTurnEvent)
