package elevenlabs

import (
	"context"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/ageneralai/maven/internal/kernel/httpc"
	pkgvoice "github.com/ageneralai/maven/internal/kernel/voice"
)

// Plugin exposes ElevenLabs TTS when speech config selects elevenlabs.
type Plugin struct{}

func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "elevenlabs" }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "elevenlabs") {
		return nil
	}
	voiceID := strings.TrimSpace(cfg.Speech.ElevenLabs.VoiceID)
	if voiceID == "" {
		return nil
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.ElevenLabs) == "" {
		return nil
	}
	httpClient, err := httpc.ClientFromProxy(cfg.Speech.ElevenLabs.Proxy)
	if err != nil {
		return nil
	}
	return &TTS{APIKey: k.ElevenLabs, VoiceID: voiceID, HTTPClient: httpClient}
}

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
