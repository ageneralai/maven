package adapter

import (
	"context"
	"fmt"
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
	out := make(chan string, 64)
	go func() {
		defer close(out)
		if a.Runtime == nil {
			return
		}
		events, err := a.Runtime.RunStream(ctx, api.Request{
			Prompt:    prompt,
			SessionID: a.SessionID,
		})
		if err != nil {
			if a.Log != nil {
				a.Log.Error("voice agent stream", "session", a.SessionID, "err", err)
			}
			if a.ErrOut != nil {
				_, _ = fmt.Fprintf(a.ErrOut, "Error: %v\n", err)
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

var _ converse.Agent = (*RuntimeAgent)(nil)
