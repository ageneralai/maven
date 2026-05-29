package channels

import (
	"context"

	"github.com/ageneralai/maven/internal/kernel/bus"
)

// InboundPreprocessor performs channel-specific setup before an agent turn.
type InboundPreprocessor interface {
	PreProcessInbound(ctx context.Context, chatID int64, hints bus.RoutingHints)
}
