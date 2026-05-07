package channel

import (
	"context"

	"github.com/ageneralai/maven/internal/bus"
)

type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Send(msg bus.OutboundMessage) error
}

type BaseChannel struct {
	name      string
	bus       *bus.MessageBus
	allowFrom map[string]bool
}

func NewBaseChannel(name string, b *bus.MessageBus, allowFrom []string) BaseChannel {
	af := make(map[string]bool, len(allowFrom))
	for _, id := range allowFrom {
		af[id] = true
	}
	return BaseChannel{name: name, bus: b, allowFrom: af}
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
