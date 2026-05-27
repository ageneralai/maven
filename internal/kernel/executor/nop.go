package executor

import "context"

// Nop implements TurnExecutor with a no-op turn (empty string, nil error).
// Use when the cron service is used for job CRUD only and never Start-ed.
type Nop struct{}

func (Nop) RunTurn(ctx context.Context, prompt, sessionID string) (string, error) {
	return "", nil
}
