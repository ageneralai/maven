package converse

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSource struct {
	ch chan Event
}

func (f *fakeSource) Listen(ctx context.Context) <-chan Event {
	out := make(chan Event, len(f.ch))
	go func() {
		defer close(out)
		for ev := range f.ch {
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()
	return out
}

type fakeSink struct {
	mu      sync.Mutex
	renders [][]string
}

func (f *fakeSink) Render(ctx context.Context, reply <-chan string) error {
	var parts []string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case s, ok := <-reply:
			if !ok {
				f.mu.Lock()
				f.renders = append(f.renders, parts)
				f.mu.Unlock()
				return nil
			}
			parts = append(parts, s)
		}
	}
}

func (f *fakeSink) snapshots() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([][]string, len(f.renders))
	copy(cp, f.renders)
	return cp
}

type fakeAgent struct {
	mu       sync.Mutex
	calls    []string
	cancelCh chan struct{}
}

func (a *fakeAgent) Stream(ctx context.Context, prompt string) <-chan string {
	a.mu.Lock()
	a.calls = append(a.calls, prompt)
	a.mu.Unlock()
	out := make(chan string, 8)
	go func() {
		defer close(out)
		select {
		case <-ctx.Done():
			a.signalCancel()
			return
		case out <- "hello":
		}
		select {
		case <-ctx.Done():
			a.signalCancel()
			return
		case out <- " world":
		}
	}()
	return out
}

func (a *fakeAgent) signalCancel() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelCh != nil {
		close(a.cancelCh)
		a.cancelCh = nil
	}
}

func (a *fakeAgent) waitCancel(timeout time.Duration) bool {
	a.mu.Lock()
	ch := a.cancelCh
	a.mu.Unlock()
	if ch == nil {
		return true
	}
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

func TestConverse_FanOutMultipleSinks(t *testing.T) {
	src := &fakeSource{ch: make(chan Event, 1)}
	sinkA := &fakeSink{}
	sinkB := &fakeSink{}
	agent := &fakeAgent{cancelCh: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Converse(ctx, []Source{src}, []Sink{sinkA, sinkB}, agent)
	}()
	src.ch <- Utterance{Text: "hi"}
	close(src.ch)
	if err := <-done; err != nil {
		t.Fatalf("Converse: %v", err)
	}
	for _, sink := range []*fakeSink{sinkA, sinkB} {
		snaps := sink.snapshots()
		if len(snaps) != 1 {
			t.Fatalf("expected 1 render, got %d", len(snaps))
		}
		if len(snaps[0]) != 2 || snaps[0][0] != "hello" || snaps[0][1] != " world" {
			t.Fatalf("unexpected render: %v", snaps[0])
		}
	}
}

func TestConverse_BargeInCancelsPreviousTurn(t *testing.T) {
	src := &fakeSource{ch: make(chan Event, 2)}
	sink := &fakeSink{}
	var firstTurnCancelled atomic.Bool
	agent := &cancelTrackingAgent{cancelled: &firstTurnCancelled}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Converse(ctx, []Source{src}, []Sink{sink}, agent)
	}()
	src.ch <- Utterance{Text: "first"}
	time.Sleep(20 * time.Millisecond)
	src.ch <- Utterance{Text: "second"}
	close(src.ch)
	if err := <-done; err != nil {
		t.Fatalf("Converse: %v", err)
	}
	if !firstTurnCancelled.Load() {
		t.Fatal("expected previous turn to be cancelled")
	}
	snaps := sink.snapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected 1 completed render, got %d", len(snaps))
	}
	if len(snaps[0]) != 1 || snaps[0][0] != "second-reply" {
		t.Fatalf("unexpected render: %v", snaps[0])
	}
}

type cancelTrackingAgent struct {
	cancelled *atomic.Bool
}

func (a *cancelTrackingAgent) Stream(ctx context.Context, prompt string) <-chan string {
	out := make(chan string, 4)
	go func() {
		defer close(out)
		if prompt == "first" {
			select {
			case <-ctx.Done():
				a.cancelled.Store(true)
				return
			case <-time.After(500 * time.Millisecond):
			}
			select {
			case <-ctx.Done():
				a.cancelled.Store(true)
				return
			case out <- "late-first":
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case out <- "second-reply":
		}
	}()
	return out
}

func TestConverse_FanInMultipleSources(t *testing.T) {
	srcA := &fakeSource{ch: make(chan Event, 1)}
	srcB := &fakeSource{ch: make(chan Event, 1)}
	sink := &fakeSink{}
	agent := &fakeAgent{cancelCh: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Converse(ctx, []Source{srcA, srcB}, []Sink{sink}, agent)
	}()
	srcA.ch <- Utterance{Text: "from-a"}
	close(srcA.ch)
	srcB.ch <- Utterance{Text: "from-b"}
	close(srcB.ch)
	if err := <-done; err != nil {
		t.Fatalf("Converse: %v", err)
	}
	agent.mu.Lock()
	calls := append([]string(nil), agent.calls...)
	agent.mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("expected 2 agent calls, got %d: %v", len(calls), calls)
	}
}

func TestConverse_SpeechStartPreemptsWithoutNewTurn(t *testing.T) {
	src := &fakeSource{ch: make(chan Event, 3)}
	sink := &fakeSink{}
	var turnCancelled atomic.Bool
	agent := &cancelTrackingAgent{cancelled: &turnCancelled}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Converse(ctx, []Source{src}, []Sink{sink}, agent)
	}()
	src.ch <- Utterance{Text: "first"}
	time.Sleep(20 * time.Millisecond)
	src.ch <- SpeechStart{}
	time.Sleep(20 * time.Millisecond)
	src.ch <- Utterance{Text: "second"}
	close(src.ch)
	if err := <-done; err != nil {
		t.Fatalf("Converse: %v", err)
	}
	if !turnCancelled.Load() {
		t.Fatal("expected in-flight turn cancelled by SpeechStart")
	}
	snaps := sink.snapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected 1 completed render after Utterance following SpeechStart, got %d", len(snaps))
	}
	if len(snaps[0]) != 1 || snaps[0][0] != "second-reply" {
		t.Fatalf("unexpected render: %v", snaps[0])
	}
}
