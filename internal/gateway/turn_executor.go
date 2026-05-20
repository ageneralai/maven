package gateway

import (
	"context"
	"fmt"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/agent"
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

// pipelineStreamRunner implements web.StreamRunner by delegating to the pipeline's runtime.
type pipelineStreamRunner struct {
	pipeFn func() *pipeline.Pipeline
}

func (r *pipelineStreamRunner) RunStream(ctx context.Context, prompt, sessionID string) (<-chan api.StreamEvent, error) {
	p := r.pipeFn()
	if p == nil {
		return nil, fmt.Errorf("gateway: pipeline not initialized")
	}
	rt := p.CurrentRuntime()
	if rt == nil {
		return nil, fmt.Errorf("gateway: runtime not initialized")
	}
	return agent.RunStream(ctx, rt, prompt, sessionID, nil)
}
