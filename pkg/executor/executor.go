package executor

import "context"

// TurnExecutor runs one agent turn and returns the text output.
// Implementations: pipeline adapter (gateway), mock (tests).
type TurnExecutor interface {
	RunTurn(ctx context.Context, prompt, sessionID string) (string, error)
}
