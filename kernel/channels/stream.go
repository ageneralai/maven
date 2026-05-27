package channels

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
)

// StreamChannel supports streaming model output to the chat surface.
type StreamChannel interface {
	Channel
	SendStream(ctx context.Context, chatID string, metadata map[string]any, events <-chan api.StreamEvent) error
}
