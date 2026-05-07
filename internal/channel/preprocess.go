package channel

import (
	"context"

	"github.com/ageneralai/maven/internal/bus"
)

// InboundPreprocessor performs channel-specific setup before an agent turn
// (e.g. reaction placeholders). Implementations are optional.
type InboundPreprocessor interface {
	PreProcessInbound(ctx context.Context, chatID int64, hints bus.RoutingHints)
}
