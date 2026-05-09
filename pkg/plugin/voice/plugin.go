package voice

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/plugin"
)

// Plugin marks voice-capable Web UI mode for registry ordering; lifecycle stays in internal/channel/webui and internal/voice.
type Plugin struct{}

func New() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "voice" }

func (Plugin) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Channels.WebUI.Enabled && cfg.Channels.WebUI.Voice.Enabled
}

func (Plugin) Tools(*config.Config) []tool.Tool { return nil }

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
