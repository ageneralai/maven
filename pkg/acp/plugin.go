package acp

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/plugin"
	"github.com/ageneralai/maven/pkg/voice"
)

// Plugin registers DelegateTask when configuration yields tools (see Tools).
type Plugin struct{}

// NewPlugin returns plugin.Plugin for gateway registration.
func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "acp" }

func (Plugin) Enabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return len(Tools(cfg.Tools.ACP, cfg.Agent.Workspace, cfg.Tools.RestrictToWorkspace)) > 0
}

func (Plugin) Tools(cfg *config.Config) []tool.Tool {
	if cfg == nil {
		return nil
	}
	return Tools(cfg.Tools.ACP, cfg.Agent.Workspace, cfg.Tools.RestrictToWorkspace)
}

func (Plugin) TTSProvider(*config.Config) voice.TTSProvider { return nil }

func (Plugin) STTProvider(*config.Config) voice.STTProvider { return nil }

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
