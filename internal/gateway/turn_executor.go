package gateway

import (
	"context"
	"fmt"

	"github.com/ageneralai/maven/internal/pipeline"
)

// gatewayTurnExecutor delegates unattended turns (cron, heartbeat) to the pipeline.
type gatewayTurnExecutor struct {
	pipeFn func() *pipeline.Pipeline
}

func (e *gatewayTurnExecutor) RunTurn(ctx context.Context, prompt, sessionID string) (string, error) {
	p := e.pipeFn()
	if p == nil {
		return "", fmt.Errorf("gateway: pipeline not initialized")
	}
	return p.RunTurn(ctx, prompt, sessionID)
}
