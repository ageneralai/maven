package agent

import (
	"context"

	"github.com/cexll/agentsdk-go/pkg/api"
)

// Runtime is the agent execution surface (typically agentsdk-go).
type Runtime interface {
	Run(ctx context.Context, req api.Request) (*api.Response, error)
	RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error)
	Close()
}
