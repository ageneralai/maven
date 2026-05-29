package adapter

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/kernel/agent"
)

type fakeRuntime struct {
	events []api.StreamEvent
	err    error
}

func (f *fakeRuntime) Run(ctx context.Context, req api.Request) (*api.Response, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeRuntime) RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error) {
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

func (f *fakeRuntime) Close() {}

func TestDeltas(t *testing.T) {
	t.Parallel()
	events := make(chan api.StreamEvent, 4)
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Text: "Hi "}}
	events <- api.StreamEvent{Type: api.EventPing}
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Text: "there."}}
	close(events)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var got []string
	for d := range Deltas(ctx, events) {
		got = append(got, d)
	}
	if len(got) != 2 || got[0] != "Hi " || got[1] != "there." {
		t.Fatalf("deltas = %#v", got)
	}
}

func TestRuntimeAgent_Stream(t *testing.T) {
	t.Parallel()
	rt := &fakeRuntime{
		events: []api.StreamEvent{
			{Type: api.EventContentBlockDelta, Delta: &api.Delta{Text: "hello"}},
			{Type: api.EventContentBlockDelta, Delta: &api.Delta{Text: " world"}},
		},
	}
	a := &RuntimeAgent{Runtime: rt, SessionID: "test"}
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

func TestRuntimeAgent_StreamError(t *testing.T) {
	t.Parallel()
	rt := &fakeRuntime{err: errors.New("boom")}
	var errBuf bytes.Buffer
	a := &RuntimeAgent{Runtime: rt, SessionID: "test", ErrOut: &errBuf}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for range a.Stream(ctx, "prompt") {
		t.Fatal("expected no deltas on error")
	}
	if !bytes.Contains(errBuf.Bytes(), []byte("boom")) {
		t.Fatalf("ErrOut = %q", errBuf.String())
	}
}

var _ agent.Runtime = (*fakeRuntime)(nil)
