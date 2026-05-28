package events

import (
	"context"
	"testing"
)

func TestOrPublisher_nilIsNoOp(t *testing.T) {
	t.Parallel()
	p := OrPublisher(nil)
	p.Publish(context.Background(), Event{Type: "x"})
}

func TestOrPublisher_passThrough(t *testing.T) {
	t.Parallel()
	n := 0
	var sub EventPublisher = callbackPublisher(func(context.Context, Event) { n++ })
	p := OrPublisher(sub)
	p.Publish(context.Background(), Event{Type: "y"})
	if n != 1 {
		t.Fatalf("want 1 publish, got %d", n)
	}
}

type callbackPublisher func(context.Context, Event)

func (f callbackPublisher) Publish(ctx context.Context, e Event) {
	f(ctx, e)
}
