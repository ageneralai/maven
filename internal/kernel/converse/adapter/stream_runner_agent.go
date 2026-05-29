package adapter

import (
	"context"
	"io"
	"log/slog"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/kernel/converse"
	"github.com/ageneralai/maven/internal/kernel/executor"
)

// StreamRunnerAgent adapts executor.StreamRunner to the converse Agent port.
type StreamRunnerAgent struct {
	Runner    executor.StreamRunner
	SessionID string
	ErrOut    io.Writer
	Log       *slog.Logger
}

func (a *StreamRunnerAgent) Stream(ctx context.Context, prompt string) <-chan string {
	cfg := streamConfig{session: a.SessionID, log: a.Log, errOut: a.ErrOut}
	return streamDeltas(ctx, cfg, func() (<-chan api.StreamEvent, error) {
		if a.Runner == nil {
			return nil, nil
		}
		return a.Runner.RunStream(ctx, prompt, a.SessionID)
	})
}

var _ converse.Agent = (*StreamRunnerAgent)(nil)
