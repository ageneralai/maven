package web

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	chann "github.com/ageneralai/maven/internal/channel"
)

var _ chann.Channel = (*WebChannel)(nil)
var _ chann.StreamChannel = (*WebChannel)(nil)

// StreamRunner runs a streaming agent turn. Implemented by the gateway pipeline.
type StreamRunner interface {
	RunStream(ctx context.Context, prompt, sessionID string) (<-chan api.StreamEvent, error)
}
