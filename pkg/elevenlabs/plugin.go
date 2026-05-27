package elevenlabs

import (
	"context"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/httpc"
	"github.com/ageneralai/maven/pkg/plugin"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

// Plugin exposes ElevenLabs TTS when speech config selects elevenlabs.
type Plugin struct{}

func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "elevenlabs" }

func (Plugin) Enabled(cfg *config.Config) bool {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "elevenlabs") {
		return false
	}
	if strings.TrimSpace(cfg.Speech.ElevenLabs.VoiceID) == "" {
		return false
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.ElevenLabs) == "" {
		return false
	}
	_, err := httpc.ClientFromProxy(cfg.Speech.ElevenLabs.Proxy)
	return err == nil
}

func (Plugin) Tools(*config.Config) []tool.Tool { return nil }

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

func (Plugin) STTProvider(*config.Config) pkgvoice.STTProvider { return nil }

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
