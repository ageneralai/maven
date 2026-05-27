package cartesia

import (
	"context"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/httpc"
	"github.com/ageneralai/maven/pkg/plugin"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

// Plugin exposes Cartesia TTS when speech config selects cartesia.
type Plugin struct{}

func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "cartesia" }

func (Plugin) Enabled(cfg *config.Config) bool {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "cartesia") {
		return false
	}
	if strings.TrimSpace(cfg.Speech.Cartesia.VoiceID) == "" {
		return false
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.Cartesia) == "" {
		return false
	}
	_, err := httpc.ClientFromProxy(cfg.Speech.Cartesia.Proxy)
	return err == nil
}

func (Plugin) Tools(*config.Config) []tool.Tool { return nil }

func (Plugin) Channels(*config.Config) []channel.Channel { return nil }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "cartesia") {
		return nil
	}
	voiceID := strings.TrimSpace(cfg.Speech.Cartesia.VoiceID)
	if voiceID == "" {
		return nil
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.Cartesia) == "" {
		return nil
	}
	httpClient, err := httpc.ClientFromProxy(cfg.Speech.Cartesia.Proxy)
	if err != nil {
		return nil
	}
	return &TTS{
		APIKey:     k.Cartesia,
		VoiceID:    voiceID,
		ModelID:    cfg.Speech.Cartesia.ModelID,
		Version:    cfg.Speech.Cartesia.APIVersion,
		HTTPClient: httpClient,
	}
}

func (Plugin) STTProvider(*config.Config) pkgvoice.STTProvider { return nil }

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
