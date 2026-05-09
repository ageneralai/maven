package acp

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
	mavenacp "github.com/ageneralai/maven/pkg/acp"
	"github.com/ageneralai/maven/pkg/plugin"
)

// Plugin registers ACP delegate_task when configuration yields tools (single source of truth: pkg/acp.Tools).
type Plugin struct{}

func New() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "acp" }

func (Plugin) Enabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return len(mavenacp.Tools(cfg.Tools.ACP, cfg.Agent.Workspace, cfg.Tools.RestrictToWorkspace)) > 0
}

func (Plugin) Tools(cfg *config.Config) []tool.Tool {
	if cfg == nil {
		return nil
	}
	return mavenacp.Tools(cfg.Tools.ACP, cfg.Agent.Workspace, cfg.Tools.RestrictToWorkspace)
}

func (Plugin) Channels(*config.Config) []channel.Channel { return nil }

func (Plugin) Provider(*config.Config) api.ModelFactory { return nil }

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
