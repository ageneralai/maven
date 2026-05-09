package plugin

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
)

// Plugin is an optional tool bundle or lifecycle hook registered with the gateway (see Registry).
type Plugin interface {
	Name() string
	Enabled(cfg *config.Config) bool
	Tools(cfg *config.Config) []tool.Tool
	Start(ctx context.Context) error
	Stop() error
}
