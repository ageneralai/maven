package cartesia

import (
	"context"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/ageneralai/maven/internal/kernel/httpc"
	pkgvoice "github.com/ageneralai/maven/internal/kernel/voice"
)

// Plugin exposes Cartesia TTS when speech config selects cartesia.
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

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
