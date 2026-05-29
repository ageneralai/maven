package terminal

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestTerminalSink_PrintsYouPromptOnNaturalCompletion(t *testing.T) {
	var buf bytes.Buffer
	s := NewSession(&buf, strings.NewReader(""))
	sink := s.Screen().(*TerminalSink)
	ctx := context.Background()
	reply := make(chan string)
	close(reply)
	if err := sink.Render(ctx, reply); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "you ▸") {
		t.Fatalf("expected you ▸ after natural completion, got %q", buf.String())
	}
}

func TestTerminalSink_NoPromptOnCancel(t *testing.T) {
	var buf bytes.Buffer
	s := NewSession(&buf, strings.NewReader(""))
	sink := s.Screen().(*TerminalSink)
	ctx, cancel := context.WithCancel(context.Background())
	reply := make(chan string, 1)
	reply <- "partial"
	cancel()
	if err := sink.Render(ctx, reply); err == nil {
		t.Fatal("expected cancel error")
	}
	if strings.Contains(buf.String(), "you ▸") {
		t.Fatalf("expected no you ▸ on cancel, got %q", buf.String())
	}
}

func TestSession_PrintYouPrompt(t *testing.T) {
	var buf bytes.Buffer
	s := NewSession(&buf, strings.NewReader(""))
	buf.Reset()
	s.PrintYouPrompt()
	if buf.String() != "\nyou ▸ " {
		t.Fatalf("got %q", buf.String())
	}
}
