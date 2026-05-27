package acp

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/kernel/config"
	"github.com/ageneralai/maven/kernel/plugin"
)

// Plugin registers DelegateTask when configuration yields tools (see Tools).
type Plugin struct{}

// NewPlugin returns a ToolPlugin for gateway registration.
func NewPlugin() plugin.ToolPlugin { return Plugin{} }

func (Plugin) Name() string { return "acp" }

func (Plugin) Tools(cfg *config.Config) []tool.Tool {
	if cfg == nil {
		return nil
	}
	return Tools(cfg.Tools.ACP, cfg.Agent.Workspace, cfg.Tools.RestrictToWorkspace)
}

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
