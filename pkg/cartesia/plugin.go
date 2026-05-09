package cartesia

import (
	"context"
	"os"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/plugin"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

// Plugin exposes Cartesia TTS when Web UI voice selects cartesia.
type Plugin struct{}

func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "cartesia" }

func (Plugin) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Channels.WebUI.Voice.Enabled
}

func (Plugin) Tools(*config.Config) []tool.Tool { return nil }

func (Plugin) Channels(*config.Config) []channel.Channel { return nil }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !cfg.Channels.WebUI.Voice.Enabled {
		return nil
	}
	if pkgvoice.NormalizeTTS(cfg.Channels.WebUI.Voice.TTSProvider) != "cartesia" {
		return nil
	}
	voiceID := strings.TrimSpace(os.Getenv("CARTESIA_VOICE_ID"))
	if voiceID == "" {
		return nil
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.Cartesia) == "" {
		return nil
	}
	cts := &TTS{APIKey: k.Cartesia, VoiceID: voiceID}
	if m := strings.TrimSpace(os.Getenv("CARTESIA_MODEL_ID")); m != "" {
		cts.ModelID = m
	}
	if v := strings.TrimSpace(os.Getenv("CARTESIA_API_VERSION")); v != "" {
		cts.Version = v
	}
	return cts
}

func (Plugin) STTProvider(*config.Config) pkgvoice.STTProvider { return nil }

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
