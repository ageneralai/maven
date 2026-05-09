package plugin

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/voice"
)

// Plugin is an optional tool bundle or lifecycle hook registered with the gateway (see Registry).
type Plugin interface {
	Name() string
	Enabled(cfg *config.Config) bool
	Tools(cfg *config.Config) []tool.Tool
	Channels(cfg *config.Config) []channel.Channel
	TTSProvider(cfg *config.Config) voice.TTSProvider
	STTProvider(cfg *config.Config) voice.STTProvider
	Start(ctx context.Context) error
	Stop() error
}
