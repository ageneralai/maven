package terminal

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/ageneralai/maven/internal/kernel/converse"
)

type callTrackingAgent struct {
	mu     sync.Mutex
	calls  []string
	callCh chan string
}

func (a *callTrackingAgent) Stream(ctx context.Context, prompt string) <-chan string {
	a.mu.Lock()
	a.calls = append(a.calls, prompt)
	a.mu.Unlock()
	select {
	case a.callCh <- prompt:
	default:
	}
	out := make(chan string, 1)
	go func() {
		defer close(out)
		if prompt == "keyboard" {
			select {
			case <-ctx.Done():
				return
			case out <- "typing...":
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case out <- "done":
		}
	}()
	return out
}

func TestVoiceBargeInDuringKeyboardTurn(t *testing.T) {
	voiceSrc := &stubSource{ch: make(chan converse.Event, 2)}
	stdinR, stdinW := io.Pipe()
	var buf bytes.Buffer
	session := NewSession(&buf, stdinR)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	agent := &callTrackingAgent{callCh: make(chan string, 4)}
	done := make(chan error, 1)
	go func() {
		done <- converse.Converse(
			ctx,
			[]converse.Source{session.Keyboard(), session.Voice(voiceSrc)},
			[]converse.Sink{session.Screen()},
			agent,
		)
	}()
	go func() {
		_, _ = io.WriteString(stdinW, "keyboard\n")
	}()
	select {
	case call := <-agent.callCh:
		if call != "keyboard" {
			t.Fatalf("expected keyboard first, got %q", call)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("keyboard turn did not start")
	}
	go func() {
		voiceSrc.ch <- converse.Utterance{Text: "voice barge"}
	}()
	select {
	case call := <-agent.callCh:
		if call != "voice barge" {
			t.Fatalf("expected voice barge, got %q", call)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("voice utterance did not reach agent within 200ms")
	}
	cancel()
	close(voiceSrc.ch)
	_ = stdinW.Close()
	<-done
}
