package events

import (
	"context"
	"testing"
)

func TestOrPublisher_nilIsNoOp(t *testing.T) {
	p := OrPublisher(nil)
	p.Publish(context.Background(), Event{Type: "x"})
}

func TestOrPublisher_passThrough(t *testing.T) {
	n := 0
	var sub EventPublisher = callbackPublisher(func(context.Context, Event) { n++ })
	p := OrPublisher(sub)
	p.Publish(context.Background(), Event{Type: "y"})
	if n != 1 {
		t.Fatalf("want 1 publish, got %d", n)
	}
}

func TestRegistry_SetDefaultPublisher_nilResetsNoOp(t *testing.T) {
	t.Cleanup(func() { SetDefaultPublisher(nil) })
	SetDefaultPublisher(nil)
	var n int
	SetDefaultPublisher(callbackPublisher(func(context.Context, Event) { n++ }))
	SetDefaultPublisher(nil)
	Publish(context.Background(), Event{Type: "z"})
	if n != 0 {
		t.Fatalf("want no publish after nil reset, got %d", n)
	}
}

func TestRegistry_PublishUsesDefault(t *testing.T) {
	t.Cleanup(func() { SetDefaultPublisher(nil) })
	var got string
	SetDefaultPublisher(callbackPublisher(func(_ context.Context, e Event) { got = e.Type }))
	Publish(context.Background(), Event{Type: "hello"})
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

type callbackPublisher func(context.Context, Event)

func (f callbackPublisher) Publish(ctx context.Context, e Event) {
	f(ctx, e)
}
