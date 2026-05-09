package events

import "context"

var defaultPublisher EventPublisher = NoOp{}

func SetDefaultPublisher(p EventPublisher) {
	if p == nil {
		defaultPublisher = NoOp{}
		return
	}
	defaultPublisher = p
}

func Publish(ctx context.Context, e Event) {
	defaultPublisher.Publish(ctx, e)
}
