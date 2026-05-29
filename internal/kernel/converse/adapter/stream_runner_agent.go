package adapter

import (
	"context"
	"log/slog"

	"github.com/ageneralai/maven/internal/kernel/converse"
	"github.com/ageneralai/maven/internal/kernel/executor"
)

// StreamRunnerAgent adapts executor.StreamRunner to the converse Agent port.
type StreamRunnerAgent struct {
	Runner    executor.StreamRunner
	SessionID string
	Log       *slog.Logger
}

func (a *StreamRunnerAgent) Stream(ctx context.Context, prompt string) <-chan string {
	out := make(chan string, 64)
	go func() {
		defer close(out)
		if a.Runner == nil {
			return
		}
		events, err := a.Runner.RunStream(ctx, prompt, a.SessionID)
		if err != nil {
			if a.Log != nil {
				a.Log.Error("voice agent stream", "session", a.SessionID, "err", err)
			}
			return
		}
		for delta := range Deltas(ctx, events) {
			select {
			case <-ctx.Done():
				logVoiceTurnInterrupted(a.Log, a.SessionID, ctx)
				return
			case out <- delta:
			}
		}
	}()
	return out
}

var _ converse.Agent = (*StreamRunnerAgent)(nil)
