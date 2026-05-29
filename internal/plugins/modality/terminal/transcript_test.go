package terminal

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ageneralai/maven/internal/kernel/converse"
)

type stubSource struct {
	ch chan converse.Event
}

func (s *stubSource) Listen(ctx context.Context) <-chan converse.Event {
	out := make(chan converse.Event, len(s.ch))
	go func() {
		defer close(out)
		for ev := range s.ch {
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()
	return out
}

func TestTranscript_SerializesWrites(t *testing.T) {
	var buf bytes.Buffer
	tx := &Transcript{Out: &buf}
	start := make(chan struct{})
	done := make(chan struct{})
	go func() {
		<-start
		_, _ = tx.Write([]byte("b"))
		close(done)
	}()
	_, _ = tx.Write([]byte("a"))
	close(start)
	<-done
	if buf.String() != "ab" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestScreenSink_Label(t *testing.T) {
	var buf bytes.Buffer
	sink := &ScreenSink{Out: &buf, Label: "maven ▸ "}
	ctx := context.Background()
	reply := make(chan string, 2)
	reply <- "hi"
	reply <- "!"
	close(reply)
	if err := sink.Render(ctx, reply); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.HasPrefix(got, "\nmaven ▸ hi!") {
		t.Fatalf("unexpected output: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected trailing newline, got %q", got)
	}
}

func TestVoiceLine_WritesTranscript(t *testing.T) {
	src := &stubSource{ch: make(chan converse.Event, 1)}
	var buf bytes.Buffer
	s := NewSession(&buf, strings.NewReader(""))
	v := s.Voice(src)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := v.Listen(ctx)
	src.ch <- converse.Utterance{Text: "hi"}
	close(src.ch)
	<-ch
	if !strings.Contains(buf.String(), "you ▸ hi") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestVoiceLine_ForwardsUtterance(t *testing.T) {
	src := &stubSource{ch: make(chan converse.Event, 1)}
	s := NewSession(&bytes.Buffer{}, strings.NewReader(""))
	v := s.Voice(src)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ch := v.Listen(ctx)
	src.ch <- converse.Utterance{Text: "x"}
	close(src.ch)
	ev, ok := <-ch
	if !ok {
		t.Fatal("expected forwarded utterance")
	}
	u, isU := ev.(converse.Utterance)
	if !isU || u.Text != "x" {
		t.Fatalf("got %+v", ev)
	}
}

func TestVoiceLine_IgnoresSpeechStart(t *testing.T) {
	src := &stubSource{ch: make(chan converse.Event, 2)}
	var buf bytes.Buffer
	s := NewSession(&buf, strings.NewReader(""))
	v := s.Voice(src)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := v.Listen(ctx)
	src.ch <- converse.SpeechStart{}
	src.ch <- converse.Utterance{Text: "hi"}
	close(src.ch)
	var got []converse.Event
	for ev := range ch {
		got = append(got, ev)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if _, ok := got[0].(converse.SpeechStart); !ok {
		t.Fatalf("first event should be SpeechStart, got %T", got[0])
	}
}
