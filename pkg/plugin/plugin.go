package plugin

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/voice"
)

// Plugin is an optional tool bundle or lifecycle hook registered with the gateway (see Registry).
type Plugin interface {
	// Name returns the plugin identifier for logs and errors.
	Name() string
	// Enabled reports whether this plugin should contribute tools or voice providers.
	Enabled(cfg *config.Config) bool
	// Tools returns agent tools when enabled.
	Tools(cfg *config.Config) []tool.Tool
	// TTSProvider returns text-to-speech when this plugin supplies one.
	// Registry.TTSProvider walks enabled plugins in registration order; the first non-nil provider wins.
	TTSProvider(cfg *config.Config) voice.TTSProvider
	// STTProvider returns speech-to-text when this plugin supplies one.
	// Registry.STTProvider walks enabled plugins in registration order; the first non-nil provider wins.
	STTProvider(cfg *config.Config) voice.STTProvider
	// Start runs plugin startup hooks before the gateway accepts traffic.
	Start(ctx context.Context) error
	// Stop runs plugin shutdown hooks during gateway teardown.
	Stop() error
}
