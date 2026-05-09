package channel

import (
	"context"

	"github.com/ageneralai/maven/internal/bus"
	mavenlog "github.com/ageneralai/maven/pkg/log"
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
// Capabilities().ReactiveOnly only support reply-path outbound; see gateway cron deliver path.
type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Send(ctx context.Context, msg bus.OutboundMessage) error
	Capabilities() CapabilitySet
}

type BaseChannel struct {
	name      string
	Bus       *bus.MessageBus
	allowFrom map[string]bool
	Log       mavenlog.PrintLogger
}

func NewBaseChannel(name string, b *bus.MessageBus, allowFrom []string, log mavenlog.PrintLogger) BaseChannel {
	af := make(map[string]bool, len(allowFrom))
	for _, id := range allowFrom {
		af[id] = true
	}
	return BaseChannel{name: name, Bus: b, allowFrom: af, Log: log}
}

func (c *BaseChannel) Name() string {
	return c.name
}

func (c *BaseChannel) IsAllowed(senderID string) bool {
	if len(c.allowFrom) == 0 {
		return true
	}
	return c.allowFrom[senderID]
}
