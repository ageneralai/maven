package bus

import "context"

// OutboundPublisher is the narrow publish surface for triggers and plugins.
type OutboundPublisher interface {
	PublishOutbound(ctx context.Context, channel, chatID, content string) error
}

// Publisher adapts MessageBus to OutboundPublisher.
type Publisher struct {
	Bus *MessageBus
}

func (p Publisher) PublishOutbound(ctx context.Context, channel, chatID, content string) error {
	if p.Bus == nil {
		return ErrBusClosed
	}
	return p.Bus.PublishOutbound(ctx, OutboundMessage{Channel: channel, ChatID: chatID, Content: content})
}
