package channels

import (
	"context"

	"github.com/ageneralai/maven/kernel/bus"
)

// CapabilitySet declares optional channel behavior. ReactiveOnly means outbound
// Send only works after a recent inbound on that chat (passive reply); proactive
// delivery (e.g. cron with deliver=true) must be skipped at the gateway.
type CapabilitySet struct {
	Reactions    bool
	FileUpload   bool
	ReactiveOnly bool
}

// Channel is a named chat transport. Send delivers to msg.ChatID.
type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Send(ctx context.Context, msg bus.OutboundMessage) error
	Capabilities() CapabilitySet
}
