package bus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ageneralai/maven/kernel/health"
	"github.com/ageneralai/maven/kernel/health/healthtest"
	"github.com/ageneralai/maven/kernel/events"
	"github.com/ageneralai/maven/kernel/events/eventsfake"
	"log/slog"
)

var testLG = slog.New(slog.DiscardHandler)

func TestNewMessageBus_Capacity(t *testing.T) {
	b := New(10, testLG)
	defer b.Close()
	if cap(b.InboundChan()) != 10 {
		t.Errorf("inbound cap = %d, want 10", cap(b.InboundChan()))
	}
	if cap(b.OutboundChan()) != 10 {
		t.Errorf("outbound cap = %d, want 10", cap(b.OutboundChan()))
	}
}

func TestNewMessageBus_DefaultSize(t *testing.T) {
	b := New(0, testLG)
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
	b := New(10, testLG)
	defer b.Close()
	err := b.PublishInbound(context.Background(), InboundMessage{Channel: "   ", ChatID: "x"})
	if !errors.Is(err, ErrInvalidInbound) {
		t.Fatalf("err = %v want ErrInvalidInbound", err)
	}
	if err := b.PublishInbound(context.Background(), InboundMessage{Channel: "ok", ChatID: "y"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestPublishOutbound_InvalidChannel(t *testing.T) {
	b := New(10, testLG)
	defer b.Close()
	err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: ""})
	if !errors.Is(err, ErrInvalidOutbound) {
		t.Fatalf("err = %v want ErrInvalidOutbound", err)
	}
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "ok", ChatID: "z"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestNormalizeInboundTrims(t *testing.T) {
	b := New(10, testLG)
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
	b := New(10, testLG)
	defer b.Close()
	var received OutboundMessage
	var mu sync.Mutex
	done := make(chan struct{})
	b.SetOutboundSubscriber("test-channel", func(msg OutboundMessage) error {
		mu.Lock()
		received = msg
		mu.Unlock()
		close(done)
		return nil
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
	b := New(10, testLG)
	defer b.Close()
	var first, second int
	b.SetOutboundSubscriber("c", func(OutboundMessage) error { first++; return nil })
	done := make(chan struct{})
	b.SetOutboundSubscriber("c", func(OutboundMessage) error {
		second++
		close(done)
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go b.DispatchOutbound(ctx)
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "c", Content: "x"}); err != nil {
		t.Fatal(err)
	}
	<-done
	cancel()
	if first != 0 || second != 1 {
		t.Fatalf("first=%d second=%d want 0,1", first, second)
	}
}

func TestDispatch_NoSubscriber(t *testing.T) {
	b := New(10, testLG)
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
	b := New(10, testLG)
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
	b := New(1, testLG)
	defer b.Close()
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "x", Content: "a"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := b.PublishOutbound(ctx, OutboundMessage{Channel: "x", Content: "b"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want Canceled got %v", err)
	}
}

func TestPublishOutbound_BufferFull_Deadline(t *testing.T) {
	b := New(1, testLG)
	defer b.Close()
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "a", ChatID: "1", Content: "x"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := b.PublishOutbound(ctx, OutboundMessage{Channel: "a", ChatID: "1", Content: "y"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded got %v", err)
	}
}

func TestClose_PublishReturnsErrBusClosed(t *testing.T) {
	b := New(10, testLG)
	b.Close()
	if err := b.PublishInbound(context.Background(), InboundMessage{Channel: "x"}); !errors.Is(err, ErrBusClosed) {
		t.Fatalf("want ErrBusClosed got %v", err)
	}
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "x"}); !errors.Is(err, ErrBusClosed) {
		t.Fatalf("want ErrBusClosed got %v", err)
	}
}

func TestClose_Concurrent(t *testing.T) {
	b := New(10, testLG)
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
	b := New(10, testLG)
	ctx, cancel := context.WithCancel(context.Background())
	go b.DispatchOutbound(ctx)
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "c", Content: "m"}); err != nil {
		t.Fatal(err)
	}
	cancel()
	b.Close()
	b.Close()
}

func TestWithEventPublisher_EmitsEvents(t *testing.T) {
	tests := []struct {
		name string
		buf  int
		run  func(t *testing.T, b *MessageBus)
		want []eventsfake.WantEvent
	}{
		{
			name: "publish_failure_outbound",
			buf:  1,
			run: func(t *testing.T, b *MessageBus) {
				t.Helper()
				if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "a", ChatID: "1", Content: "x"}); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				_ = b.PublishOutbound(ctx, OutboundMessage{Channel: "a", ChatID: "1", Content: "y"})
			},
			want: []eventsfake.WantEvent{{
				Type:  events.EventBusPublishFailure,
				Attrs: map[string]string{"stream": "outbound", "channel": "a"},
			}},
		},
		{
			name: "close",
			buf:  10,
			run: func(t *testing.T, b *MessageBus) {
				t.Helper()
				b.Close()
			},
			want: []eventsfake.WantEvent{{Type: events.EventBusClosed}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := &eventsfake.CapturePublisher{}
			b := New(tt.buf, testLG, WithEventPublisher(cap))
			if tt.name != "close" {
				defer b.Close()
			}
			tt.run(t, b)
			eventsfake.AssertContainsPublished(t, cap, tt.want)
		})
	}
}

func TestDispatch_SubscriberErrorEmitsDeliveryFailed(t *testing.T) {
	capture := &eventsfake.CapturePublisher{}
	var rec healthtest.PulseRecorder
	b := New(10, testLG, WithEventPublisher(capture), WithHealthReporter(&rec))
	defer b.Close()
	sendErr := errors.New("telegram down")
	b.SetOutboundSubscriber("telegram", func(OutboundMessage) error {
		return sendErr
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go b.DispatchOutbound(ctx)
	if err := b.PublishOutbound(context.Background(), OutboundMessage{
		Channel: "telegram",
		ChatID:  "1",
		Content: "hi",
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	evts := capture.Snapshot()
	var found bool
	for _, e := range evts {
		if e.Type == events.EventOutboundDeliveryFailed && e.Attrs["channel"] == "telegram" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want %s for telegram, got %#v", events.EventOutboundDeliveryFailed, evts)
	}
	if !rec.Has(health.SignalDeliveryFailed) {
		t.Fatalf("want %s pulse", health.SignalDeliveryFailed)
	}
}

func TestWithEventPublisher_NilMeansNoOp(t *testing.T) {
	b := New(2, testLG, WithEventPublisher(nil))
	defer b.Close()
	if err := b.PublishOutbound(context.Background(), OutboundMessage{Channel: "x", Content: "a"}); err != nil {
		t.Fatal(err)
	}
}

type recordingStreamDelegate struct {
	begins int
	ends   int
	last   error
}

func (r *recordingStreamDelegate) NotifyStreamBegin(ctx context.Context, _ StreamHints) context.Context {
	r.begins++
	return ctx
}

func (r *recordingStreamDelegate) NotifyStreamEnd(_ context.Context, _ StreamHints, err error) {
	r.ends++
	r.last = err
}

func TestWithStreamDelegate_OnStreamBegin_OnStreamEnd(t *testing.T) {
	d := &recordingStreamDelegate{}
	b := New(2, testLG, WithStreamDelegate(d))
	defer b.Close()
	h := StreamHints{Channel: "telegram", ChatID: "c1"}
	ctx := context.Background()
	sctx := b.OnStreamBegin(ctx, h)
	if d.begins != 1 {
		t.Fatalf("begins = %d want 1", d.begins)
	}
	if sctx != ctx {
		t.Fatal("delegate should pass through context when unchanged")
	}
	wantErr := errors.New("boom")
	b.OnStreamEnd(sctx, h, wantErr)
	if d.ends != 1 || !errors.Is(d.last, wantErr) {
		t.Fatalf("ends = %d last = %v", d.ends, d.last)
	}
}

func TestSetStreamDelegate_replacesPrevious(t *testing.T) {
	d1 := &recordingStreamDelegate{}
	d2 := &recordingStreamDelegate{}
	b := New(2, testLG, WithStreamDelegate(d1))
	defer b.Close()
	b.SetStreamDelegate(d2)
	b.OnStreamBegin(context.Background(), StreamHints{})
	if d1.begins != 0 || d2.begins != 1 {
		t.Fatalf("d1 begins=%d d2 begins=%d", d1.begins, d2.begins)
	}
}

func TestSetStreamDelegate_nilNoOp(t *testing.T) {
	d := &recordingStreamDelegate{}
	b := New(2, testLG, WithStreamDelegate(d))
	defer b.Close()
	b.SetStreamDelegate(nil)
	h := StreamHints{Channel: "x", ChatID: "y"}
	b.OnStreamBegin(context.Background(), h)
	b.OnStreamEnd(context.Background(), h, nil)
	if d.begins != 0 || d.ends != 0 {
		t.Fatalf("after noop replace: begins=%d ends=%d want 0,0", d.begins, d.ends)
	}
}

type streamWrapCtxKey struct{}

type wrapCtxStreamDel struct{}

func (wrapCtxStreamDel) NotifyStreamBegin(ctx context.Context, _ StreamHints) context.Context {
	return context.WithValue(ctx, streamWrapCtxKey{}, 42)
}

func (wrapCtxStreamDel) NotifyStreamEnd(context.Context, StreamHints, error) {}

func TestOnStreamBegin_delegateWrapsContext(t *testing.T) {
	b := New(2, testLG, WithStreamDelegate(wrapCtxStreamDel{}))
	defer b.Close()
	out := b.OnStreamBegin(context.Background(), StreamHints{})
	got, ok := out.Value(streamWrapCtxKey{}).(int)
	if !ok || got != 42 {
		t.Fatalf("value = %v ok=%v want 42,true", got, ok)
	}
}

func TestWithStreamDelegate_NilMeansNoOp(t *testing.T) {
	b := New(2, testLG, WithStreamDelegate(nil))
	defer b.Close()
	_ = b.OnStreamBegin(context.Background(), StreamHints{})
	b.OnStreamEnd(context.Background(), StreamHints{}, errors.New("e"))
}
