package web

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
)

// StreamRunner runs a streaming agent turn. Implemented by the gateway pipeline.
type StreamRunner interface {
	RunStream(ctx context.Context, prompt, sessionID string) (<-chan api.StreamEvent, error)
}
