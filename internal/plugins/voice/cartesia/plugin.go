package cartesia

import (
	"context"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/httpc"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	pkgvoice "github.com/ageneralai/maven/internal/kernel/voice"
)

type Plugin struct{}

func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "cartesia" }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "cartesia") {
		return nil
	}
	voiceID := strings.TrimSpace(cfg.Speech.Cartesia.VoiceID)
	if voiceID == "" {
		return nil
	}
	apiKey := strings.TrimSpace(cfg.Speech.Cartesia.APIKey)
	if apiKey == "" {
		return nil
	}
	httpClient, err := httpc.ClientFromProxy(cfg.Speech.Cartesia.Proxy)
	if err != nil {
		return nil
	}
	return &TTS{
		APIKey:     apiKey,
		VoiceID:    voiceID,
		ModelID:    cfg.Speech.Cartesia.ModelID,
		Version:    cfg.Speech.Cartesia.APIVersion,
		HTTPClient: httpClient,
	}
}

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
