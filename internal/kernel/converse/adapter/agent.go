package adapter

import (
	"context"
	"io"
	"log/slog"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/kernel/agent"
	"github.com/ageneralai/maven/internal/kernel/converse"
)

// RuntimeAgent adapts agent.Runtime to the converse Agent port via RunStream deltas.
type RuntimeAgent struct {
	Runtime   agent.Runtime
	SessionID string
	ErrOut    io.Writer
	Log       *slog.Logger
}

func (a *RuntimeAgent) Stream(ctx context.Context, prompt string) <-chan string {
	cfg := streamConfig{session: a.SessionID, log: a.Log, errOut: a.ErrOut}
	return streamDeltas(ctx, cfg, func() (<-chan api.StreamEvent, error) {
		if a.Runtime == nil {
			return nil, nil
		}
		return a.Runtime.RunStream(ctx, api.Request{
			Prompt:    prompt,
			SessionID: a.SessionID,
		})
	})
}

var _ converse.Agent = (*RuntimeAgent)(nil)
