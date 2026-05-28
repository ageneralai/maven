package events

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFanout_SubscribeDispatchesInGoroutine(t *testing.T) {
	t.Parallel()
	f := NewFanout(nil)
	var n atomic.Int32
	done := make(chan struct{})
	unsub := f.Subscribe(EventTurnCompleted, func(_ context.Context, e Event) {
		if e.Type != EventTurnCompleted {
			t.Errorf("type %q", e.Type)
		}
		if _, ok := e.Payload.(TurnCompleted); !ok {
			t.Error("payload type")
		}
		n.Add(1)
		close(done)
	})
	defer unsub()
	f.Publish(context.Background(), Event{
		Type:    EventTurnCompleted,
		Payload: TurnCompleted{UserMsg: "u", AssistantMsg: "a"},
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber not invoked")
	}
	if n.Load() != 1 {
		t.Fatalf("count %d", n.Load())
	}
}

func TestFanout_Unsubscribe(t *testing.T) {
	t.Parallel()
	f := NewFanout(nil)
	var n atomic.Int32
	unsub := f.Subscribe(EventTurnCompleted, func(context.Context, Event) { n.Add(1) })
	unsub()
	f.Publish(context.Background(), Event{Type: EventTurnCompleted})
	time.Sleep(50 * time.Millisecond)
	if n.Load() != 0 {
		t.Fatalf("want 0 after unsubscribe, got %d", n.Load())
	}
}

func TestFanout_UnsubscribeFirstOfTwo(t *testing.T) {
	t.Parallel()
	f := NewFanout(nil)
	var n1, n2 atomic.Int32
	unsub1 := f.Subscribe(EventTurnCompleted, func(context.Context, Event) { n1.Add(1) })
	unsub2 := f.Subscribe(EventTurnCompleted, func(context.Context, Event) { n2.Add(1) })
	unsub1()
	f.Publish(context.Background(), Event{Type: EventTurnCompleted})
	time.Sleep(50 * time.Millisecond)
	if n1.Load() != 0 {
		t.Fatalf("first subscriber fired after unsubscribe, got %d", n1.Load())
	}
	if n2.Load() != 1 {
		t.Fatalf("second subscriber want 1, got %d", n2.Load())
	}
	unsub2()
	f.Publish(context.Background(), Event{Type: EventTurnCompleted})
	time.Sleep(50 * time.Millisecond)
	if n2.Load() != 1 {
		t.Fatalf("second subscriber fired after its unsubscribe, got %d", n2.Load())
	}
}

func TestFanout_ForwardsToSink(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var got []string
	sink := callbackPublisher(func(_ context.Context, e Event) {
		mu.Lock()
		got = append(got, e.Type)
		mu.Unlock()
	})
	f := NewFanout(sink)
	f.Publish(context.Background(), Event{Type: EventStreamFailed})
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0] != EventStreamFailed {
		t.Fatalf("sink got %v", got)
	}
}
