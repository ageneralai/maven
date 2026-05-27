package channel

import (
	"context"

	"github.com/ageneralai/maven/internal/bus"
)

// CapabilitySet declares optional channel behavior. ReactiveOnly means outbound
// Send only works after a recent inbound on that chat (passive reply); proactive
// delivery (e.g. cron with deliver=true) must be skipped at the gateway.
type CapabilitySet struct {
	Reactions    bool
	FileUpload   bool
	ReactiveOnly bool
}

// Channel is a named chat transport. Send delivers to msg.ChatID. Channels with
// Capabilities().ReactiveOnly only support reply-path outbound; see cron.Deliver.
type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Send(ctx context.Context, msg bus.OutboundMessage) error
	Capabilities() CapabilitySet
}
