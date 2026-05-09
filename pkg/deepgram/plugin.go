package deepgram

import (
	"context"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/plugin"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

// Plugin exposes Deepgram STT/TTS when Web UI voice selects deepgram.
type Plugin struct{}

func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "deepgram" }

func (Plugin) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Channels.WebUI.Voice.Enabled
}

func (Plugin) Tools(*config.Config) []tool.Tool { return nil }

func (Plugin) Channels(*config.Config) []channel.Channel { return nil }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !cfg.Channels.WebUI.Voice.Enabled {
		return nil
	}
	if pkgvoice.NormalizeTTS(cfg.Channels.WebUI.Voice.TTSProvider) != "deepgram" {
		return nil
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.Deepgram) == "" {
		return nil
	}
	return &TTS{APIKey: k.Deepgram}
}

func (Plugin) STTProvider(cfg *config.Config) pkgvoice.STTProvider {
	if cfg == nil || !cfg.Channels.WebUI.Voice.Enabled {
		return nil
	}
	if pkgvoice.NormalizeSTT(cfg.Channels.WebUI.Voice.STTProvider) != "deepgram" {
		return nil
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.Deepgram) == "" {
		return nil
	}
	return &STT{APIKey: k.Deepgram}
}

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
