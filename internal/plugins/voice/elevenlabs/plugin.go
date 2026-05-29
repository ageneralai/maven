package elevenlabs

import (
	"context"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/httpc"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	pkgvoice "github.com/ageneralai/maven/internal/kernel/voice"
)

type Plugin struct{}

func NewPlugin() Plugin { return Plugin{} }

func (Plugin) Name() string { return "elevenlabs" }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "elevenlabs") {
		return nil
	}
	voiceID := strings.TrimSpace(cfg.Speech.ElevenLabs.VoiceID)
	if voiceID == "" {
		return nil
	}
	apiKey := strings.TrimSpace(cfg.Speech.ElevenLabs.APIKey)
	if apiKey == "" {
		return nil
	}
	httpClient, err := httpc.ClientFromProxy(cfg.Speech.ElevenLabs.Proxy)
	if err != nil {
		return nil
	}
	return &TTS{APIKey: apiKey, VoiceID: voiceID, HTTPClient: httpClient}
}

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }

var _ plugin.TTSPlugin = Plugin{}
