package channel

import (
	"context"

	"github.com/ageneralai/maven/internal/bus"
	mavenlog "github.com/ageneralai/maven/internal/log"
)

// CapabilitySet declares what optional behaviors a channel supports.
type CapabilitySet struct {
	Reactions  bool
	FileUpload bool
}

type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Send(ctx context.Context, msg bus.OutboundMessage) error
	Capabilities() CapabilitySet
}

type BaseChannel struct {
	name      string
	bus       *bus.MessageBus
	allowFrom map[string]bool
	log       mavenlog.PrintLogger
}

func NewBaseChannel(name string, b *bus.MessageBus, allowFrom []string, log mavenlog.PrintLogger) BaseChannel {
	af := make(map[string]bool, len(allowFrom))
	for _, id := range allowFrom {
		af[id] = true
	}
	return BaseChannel{name: name, bus: b, allowFrom: af, log: log}
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
