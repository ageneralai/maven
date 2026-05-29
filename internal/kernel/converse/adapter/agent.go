package adapter

import (
	"context"
	"io"
	"log/slog"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/kernel/converse"
)

// Agent adapts a RunStream-shaped opener to the converse Agent port.
type Agent struct {
	session string
	log     *slog.Logger
	errOut  io.Writer
	open    func(context.Context, string) (<-chan api.StreamEvent, error)
}

// NewAgent builds a converse Agent from a per-prompt stream opener.
func NewAgent(session string, log *slog.Logger, errOut io.Writer, open func(context.Context, string) (<-chan api.StreamEvent, error)) *Agent {
	return &Agent{session: session, log: log, errOut: errOut, open: open}
}

func (a *Agent) Stream(ctx context.Context, prompt string) <-chan string {
	cfg := streamConfig{session: a.session, log: a.log, errOut: a.errOut}
	return streamDeltas(ctx, cfg, func() (<-chan api.StreamEvent, error) {
		if a.open == nil {
			return nil, nil
		}
		return a.open(ctx, prompt)
	})
}

var _ converse.Agent = (*Agent)(nil)
