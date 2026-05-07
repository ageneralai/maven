package bus

import (
	"context"
	"sync"
	"testing"
	"time"

	mavenlog "github.com/ageneralai/maven/internal/log"
)

var testLG = mavenlog.Std()

func TestNewMessageBus(t *testing.T) {
	b := NewMessageBus(10, testLG)
	if b == nil {
		t.Fatal("NewMessageBus returned nil")
	}
	if cap(b.Inbound) != 10 {
		t.Errorf("inbound cap = %d, want 10", cap(b.Inbound))
	}
	if cap(b.Outbound) != 10 {
		t.Errorf("outbound cap = %d, want 10", cap(b.Outbound))
	}
}

func TestNewMessageBus_DefaultSize(t *testing.T) {
	b := NewMessageBus(0, testLG)
	if cap(b.Inbound) != 100 {
		t.Errorf("inbound cap = %d, want 100", cap(b.Inbound))
	}
}

func TestInboundMessage_StableRouteKey(t *testing.T) {
	msg := InboundMessage{Channel: "telegram", ChatID: "12345"}
	if msg.StableRouteKey() != "telegram:12345" {
		t.Errorf("StableRouteKey = %q, want telegram:12345", msg.StableRouteKey())
	}
}

func TestSubscribeAndDispatch(t *testing.T) {
	b := NewMessageBus(10, testLG)

	var received OutboundMessage
	var mu sync.Mutex
	done := make(chan struct{})

	b.SetOutboundSubscriber("test-channel", func(msg OutboundMessage) {
		mu.Lock()
		received = msg
		mu.Unlock()
		close(done)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go b.DispatchOutbound(ctx)

	b.Outbound <- OutboundMessage{
		Channel: "test-channel",
		ChatID:  "chat-1",
		Content: "hello",
	}

	select {
	case <-done:
		mu.Lock()
		defer mu.Unlock()
		if received.Content != "hello" {
			t.Errorf("content = %q, want hello", received.Content)
		}
		if received.ChatID != "chat-1" {
			t.Errorf("chatID = %q, want chat-1", received.ChatID)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for dispatch")
	}
}

func TestSetOutboundSubscriber_Replaces(t *testing.T) {
	b := NewMessageBus(10, testLG)
	var first, second int
	b.SetOutboundSubscriber("c", func(OutboundMessage) { first++ })
	b.SetOutboundSubscriber("c", func(OutboundMessage) { second++ })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go b.DispatchOutbound(ctx)
	b.Outbound <- OutboundMessage{Channel: "c", Content: "x"}
	time.Sleep(20 * time.Millisecond)
	cancel()
	if first != 0 || second != 1 {
		t.Fatalf("first=%d second=%d want 0,1", first, second)
	}
}

func TestDispatch_NoSubscriber(t *testing.T) {
	b := NewMessageBus(10, testLG)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go b.DispatchOutbound(ctx)

	// Send to channel with no subscriber - should not panic
	b.Outbound <- OutboundMessage{
		Channel: "nonexistent",
		Content: "dropped",
	}

	<-ctx.Done()
}

func TestDispatch_ContextCancel(t *testing.T) {
	b := NewMessageBus(10, testLG)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		b.DispatchOutbound(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// OK - dispatch exited
	case <-time.After(2 * time.Second):
		t.Fatal("DispatchOutbound did not exit after context cancel")
	}
}
