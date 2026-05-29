package adapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/kernel/executor"
)

type fakeStreamRunner struct {
	events []api.StreamEvent
	err    error
}

func (f *fakeStreamRunner) RunStream(ctx context.Context, prompt, sessionID string) (<-chan api.StreamEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(chan api.StreamEvent, len(f.events))
	go func() {
		defer close(out)
		for _, ev := range f.events {
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()
	return out, nil
}

func TestStreamRunnerAgent_Stream(t *testing.T) {
	t.Parallel()
	runner := &fakeStreamRunner{
		events: []api.StreamEvent{
			{Type: api.EventContentBlockDelta, Delta: &api.Delta{Text: "hello"}},
			{Type: api.EventContentBlockDelta, Delta: &api.Delta{Text: " world"}},
		},
	}
	a := &StreamRunnerAgent{Runner: runner, SessionID: "sess-1"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var got []string
	for d := range a.Stream(ctx, "prompt") {
		got = append(got, d)
	}
	if len(got) != 2 || got[0] != "hello" || got[1] != " world" {
		t.Fatalf("stream = %#v", got)
	}
}

func TestStreamRunnerAgent_StreamError(t *testing.T) {
	t.Parallel()
	runner := &fakeStreamRunner{err: errors.New("boom")}
	a := &StreamRunnerAgent{Runner: runner, SessionID: "sess-1"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for range a.Stream(ctx, "prompt") {
		t.Fatal("expected no deltas on error")
	}
}

var _ executor.StreamRunner = (*fakeStreamRunner)(nil)
