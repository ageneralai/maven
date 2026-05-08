package bus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	mavenlog "github.com/ageneralai/maven/pkg/log"
)

var testLG = mavenlog.Std()

func TestNewMessageBus_Capacity(t *testing.T) {
	b := NewMessageBus(10, testLG)
	defer b.Close()
	if cap(b.InboundChan()) != 10 {
		t.Errorf("inbound cap = %d, want 10", cap(b.InboundChan()))
	}
	if cap(b.OutboundChan()) != 10 {
		t.Errorf("outbound cap = %d, want 10", cap(b.OutboundChan()))
	}
}

func TestNewMessageBus_DefaultSize(t *testing.T) {
	b := NewMessageBus(0, testLG)
	defer b.Close()
	if cap(b.InboundChan()) != 100 {
		t.Errorf("inbound cap = %d, want 100", cap(b.InboundChan()))
	}
}

func TestInboundMessage_StableRouteKey(t *testing.T) {
	msg := InboundMessage{Channel: "telegram", ChatID: "12345"}
	if msg.StableRouteKey() != "telegram:12345" {
		t.Errorf("StableRouteKey = %q, want telegram:12345", msg.StableRouteKey())
	}
}

func TestPublishInbound_InvalidChannel(t *testing.T) {
	b := NewMessageBus(10, testLG)
	defer b.Close()
	err := b.PublishInbound(context.Background(), InboundMessage{Channel: "   ", ChatID: "x"})
	if err != ErrInvalidInbound {
		t.Fatalf("err = %v want ErrInvalidInbound", err)
	}
	if err := b.PublishInbound(context.Background(), InboundMessage{Channel: "ok", ChatID: "y"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestPublishOutbound_InvalidChannel(t *testing.T) {
	b := NewMessageBus(10, testLG)
	defer b.Close()
	err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: ""})
	if err != ErrInvalidOutbound {
		t.Fatalf("err = %v want ErrInvalidOutbound", err)
	}
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "ok", ChatID: "z"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestNormalizeInboundTrims(t *testing.T) {
	b := NewMessageBus(10, testLG)
	defer b.Close()
	done := make(chan struct{})
	go func() {
		msg := <-b.InboundChan()
		if msg.Channel != "feishu" || msg.ChatID != "c1" || msg.SenderID != "u1" {
			t.Errorf("got %+v want trimmed fields", msg)
		}
		close(done)
	}()
	if err := b.PublishInbound(context.Background(), InboundMessage{
		Channel: "  feishu  ", ChatID: " c1 ", SenderID: "\tu1\n",
	}); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestSubscribeAndDispatch(t *testing.T) {
	b := NewMessageBus(10, testLG)
	defer b.Close()
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
	if err := b.PublishOutbound(context.Background(), OutboundMessage{
		Channel: "test-channel",
		ChatID:  "chat-1",
		Content: "hello",
	}); err != nil {
		t.Fatal(err)
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
	defer b.Close()
	var first, second int
	b.SetOutboundSubscriber("c", func(OutboundMessage) { first++ })
	b.SetOutboundSubscriber("c", func(OutboundMessage) { second++ })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go b.DispatchOutbound(ctx)
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "c", Content: "x"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	cancel()
	if first != 0 || second != 1 {
		t.Fatalf("first=%d second=%d want 0,1", first, second)
	}
}

func TestDispatch_NoSubscriber(t *testing.T) {
	b := NewMessageBus(10, testLG)
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go b.DispatchOutbound(ctx)
	if err := b.PublishOutbound(context.Background(), OutboundMessage{
		Channel: "nonexistent",
		Content: "dropped",
	}); err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
}

func TestDispatch_ContextCancel(t *testing.T) {
	b := NewMessageBus(10, testLG)
	defer b.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		b.DispatchOutbound(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("DispatchOutbound did not exit after context cancel")
	}
}

func TestPublish_ContextCancel(t *testing.T) {
	b := NewMessageBus(1, testLG)
	defer b.Close()
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "x", Content: "a"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := b.PublishOutbound(ctx, OutboundMessage{Channel: "x", Content: "b"})
	if err != context.Canceled {
		t.Fatalf("want Canceled got %v", err)
	}
}

func TestPublishOutbound_BufferFull_Deadline(t *testing.T) {
	b := NewMessageBus(1, testLG)
	defer b.Close()
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "a", ChatID: "1", Content: "x"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := b.PublishOutbound(ctx, OutboundMessage{Channel: "a", ChatID: "1", Content: "y"})
	if err != context.DeadlineExceeded {
		t.Fatalf("want DeadlineExceeded got %v", err)
	}
}

func TestClose_PublishReturnsErrBusClosed(t *testing.T) {
	b := NewMessageBus(10, testLG)
	b.Close()
	if err := b.PublishInbound(context.Background(), InboundMessage{Channel: "x"}); !errors.Is(err, ErrBusClosed) {
		t.Fatalf("want ErrBusClosed got %v", err)
	}
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "x"}); !errors.Is(err, ErrBusClosed) {
		t.Fatalf("want ErrBusClosed got %v", err)
	}
}

func TestClose_Concurrent(t *testing.T) {
	b := NewMessageBus(10, testLG)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Close()
		}()
	}
	wg.Wait()
}

func TestClose_IdempotentWithDispatch(t *testing.T) {
	b := NewMessageBus(10, testLG)
	ctx, cancel := context.WithCancel(context.Background())
	go b.DispatchOutbound(ctx)
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "c", Content: "m"}); err != nil {
		t.Fatal(err)
	}
	cancel()
	b.Close()
	b.Close()
}
