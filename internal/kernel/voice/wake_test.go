package voice

import (
	"context"
	"testing"
	"time"

	"github.com/ageneralai/maven/internal/kernel/converse"
)

type fakeSource struct{ ch chan converse.Event }

func (f *fakeSource) Listen(context.Context) <-chan converse.Event { return f.ch }

func recv(t *testing.T, ch <-chan converse.Event) (converse.Event, bool) {
	t.Helper()
	select {
	case ev, ok := <-ch:
		return ev, ok
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
		return nil, false
	}
}

func expectNone(t *testing.T, ch <-chan converse.Event) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("expected no event, got %#v", ev)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestNewWakeGate_EmptyPhraseIsIdentity(t *testing.T) {
	src := &fakeSource{ch: make(chan converse.Event)}
	if got := NewWakeGate(src, "", time.Second, nil, nil); got != converse.Source(src) {
		t.Fatalf("empty phrase must return src unchanged")
	}
	if got := NewWakeGate(src, "  ,. ", time.Second, nil, nil); got != converse.Source(src) {
		t.Fatalf("punctuation-only phrase must return src unchanged")
	}
}

func TestNormalizeWake(t *testing.T) {
	cases := map[string]string{
		"Hey, Maven.":    "hey maven",
		"  HEY   maven ": "hey maven",
		"hey-maven":      "heymaven",
		"":               "",
	}
	for in, want := range cases {
		if got := normalizeWake(in); got != want {
			t.Errorf("normalizeWake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchWake(t *testing.T) {
	rem, ok := matchWake("Hey, Maven, email Bob", "hey maven")
	if !ok || rem != "email Bob" {
		t.Fatalf("got (%q,%v), want (email Bob,true)", rem, ok)
	}
	rem, ok = matchWake("hey maven", "hey maven")
	if !ok || rem != "" {
		t.Fatalf("phrase-only: got (%q,%v), want (\"\",true)", rem, ok)
	}
	rem, ok = matchWake("Hey, Maven.", "hey maven")
	if !ok || rem != "" {
		t.Fatalf("STT-punctuated phrase-only: got (%q,%v), want (\"\",true)", rem, ok)
	}
	if _, ok := matchWake("what time is it", "hey maven"); ok {
		t.Fatal("non-matching utterance must not match")
	}
	if _, ok := matchWake("hey", "hey maven"); ok {
		t.Fatal("too-short utterance must not match")
	}
}

func TestWakeGate_GatesUntilPhraseThenWindows(t *testing.T) {
	src := &fakeSource{ch: make(chan converse.Event, 1)}
	replyDone := make(chan struct{})
	out := NewWakeGate(src, "hey maven", 200*time.Millisecond, replyDone, nil).Listen(context.Background())
	src.ch <- converse.Utterance{Text: "what time is it"}
	expectNone(t, out)
	src.ch <- converse.Utterance{Text: "hey maven"}
	ev, _ := recv(t, out)
	if u, ok := ev.(converse.Utterance); !ok || u.Text != "hey maven" {
		t.Fatalf("phrase-only wake must forward the greeting: %#v", ev)
	}
	replyDone <- struct{}{}
	src.ch <- converse.Utterance{Text: "what time is it"}
	ev, _ = recv(t, out)
	if u, ok := ev.(converse.Utterance); !ok || u.Text != "what time is it" {
		t.Fatalf("active utterance not forwarded: %#v", ev)
	}
	replyDone <- struct{}{}
	time.Sleep(300 * time.Millisecond)
	src.ch <- converse.Utterance{Text: "still here"}
	expectNone(t, out)
}

func TestWakeGate_OneBreathCommand(t *testing.T) {
	src := &fakeSource{ch: make(chan converse.Event, 1)}
	out := NewWakeGate(src, "hey maven", time.Second, nil, nil).Listen(context.Background())
	src.ch <- converse.Utterance{Text: "Hey Maven what is the weather"}
	ev, _ := recv(t, out)
	if u, ok := ev.(converse.Utterance); !ok || u.Text != "what is the weather" {
		t.Fatalf("one-breath remainder not forwarded: %#v", ev)
	}
}

func TestWakeGate_ReplyDoneExtendsWindow(t *testing.T) {
	src := &fakeSource{ch: make(chan converse.Event, 1)}
	replyDone := make(chan struct{})
	out := NewWakeGate(src, "hey maven", 150*time.Millisecond, replyDone, nil).Listen(context.Background())
	src.ch <- converse.Utterance{Text: "hey maven"}
	if ev, _ := recv(t, out); ev == nil {
		t.Fatal("expected wake greeting")
	}
	replyDone <- struct{}{}
	for i := 0; i < 3; i++ {
		time.Sleep(100 * time.Millisecond)
		replyDone <- struct{}{}
	}
	src.ch <- converse.Utterance{Text: "still talking"}
	ev, _ := recv(t, out)
	if u, ok := ev.(converse.Utterance); !ok || u.Text != "still talking" {
		t.Fatalf("reply-done kicks must hold the window open: %#v", ev)
	}
}

func TestWakeGate_TimerPausedWhileAwaitingReply(t *testing.T) {
	src := &fakeSource{ch: make(chan converse.Event, 1)}
	replyDone := make(chan struct{})
	out := NewWakeGate(src, "hey maven", 100*time.Millisecond, replyDone, nil).Listen(context.Background())
	src.ch <- converse.Utterance{Text: "hey maven tell me a story"}
	if ev, _ := recv(t, out); ev == nil {
		t.Fatal("expected wake turn")
	}
	time.Sleep(250 * time.Millisecond)
	src.ch <- converse.Utterance{Text: "wait stop"}
	ev, _ := recv(t, out)
	if u, ok := ev.(converse.Utterance); !ok || u.Text != "wait stop" {
		t.Fatalf("barge-in must forward while awaiting reply: %#v", ev)
	}
	replyDone <- struct{}{}
	time.Sleep(150 * time.Millisecond)
	src.ch <- converse.Utterance{Text: "after close"}
	expectNone(t, out)
}
