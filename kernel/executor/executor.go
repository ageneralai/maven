package executor

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
)

// TurnExecutor runs one agent turn and returns the text output.
// Implementations: pipeline (gateway), mock (tests).
type TurnExecutor interface {
	RunTurn(ctx context.Context, prompt, sessionID string) (string, error)
}

// StreamRunner runs a streaming agent turn. Implemented by the gateway pipeline.
type StreamRunner interface {
	RunStream(ctx context.Context, prompt, sessionID string) (<-chan api.StreamEvent, error)
}
