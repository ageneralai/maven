package voice

import (
	"fmt"
	"os"
	"strings"

	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/plugin"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

func resolveRegistry(reg *plugin.Registry) *plugin.Registry {
	if reg != nil {
		return reg
	}
	return DefaultVoiceRegistry()
}

// NewSTT builds STT from full config and plugin registry (gateway registry or DefaultVoiceRegistry).
func NewSTT(cfg *config.Config, reg *plugin.Registry) (pkgvoice.STT, error) {
	if cfg == nil {
		return nil, fmt.Errorf("voice: nil config")
	}
	vc := cfg.Channels.Web.Voice
	if !vc.Enabled {
		return nil, fmt.Errorf("voice: web voice not enabled")
	}
	reg = resolveRegistry(reg)
	if stt := reg.STTProvider(cfg); stt != nil {
		return stt, nil
	}
	want := pkgvoice.NormalizeSTT(vc.STTProvider)
	switch want {
	case "openai":
		return nil, fmt.Errorf("voice: sttProvider %q is not supported yet; use %q", vc.STTProvider, "deepgram")
	case "deepgram":
		k := pkgvoice.MergeKeys(cfg)
		if strings.TrimSpace(k.Deepgram) == "" {
			return nil, fmt.Errorf("voice: deepgram api key is empty")
		}
	default:
		return nil, fmt.Errorf("voice: unknown sttProvider %q", vc.STTProvider)
	}
	return nil, fmt.Errorf("voice: stt provider %q failed to initialize", want)
}

// NewTTS builds TTS from full config and plugin registry.
func NewTTS(cfg *config.Config, reg *plugin.Registry) (pkgvoice.TTS, error) {
	if cfg == nil {
		return nil, fmt.Errorf("voice: nil config")
	}
	vc := cfg.Channels.Web.Voice
	if !vc.Enabled {
		return nil, fmt.Errorf("voice: web voice not enabled")
	}
	reg = resolveRegistry(reg)
	if tts := reg.TTSProvider(cfg); tts != nil {
		return tts, nil
	}
	want := pkgvoice.NormalizeTTS(vc.TTSProvider)
	switch want {
	case "deepgram":
		k := pkgvoice.MergeKeys(cfg)
		if strings.TrimSpace(k.Deepgram) == "" {
			return nil, fmt.Errorf("voice: deepgram api key is empty")
		}
	case "openai":
		k := pkgvoice.MergeKeys(cfg)
		if strings.TrimSpace(k.OpenAI) == "" {
			return nil, fmt.Errorf("voice: openai api key is empty")
		}
	case "elevenlabs":
		if strings.TrimSpace(os.Getenv("ELEVENLABS_VOICE_ID")) == "" {
			return nil, fmt.Errorf("voice: ELEVENLABS_VOICE_ID is required for elevenlabs tts")
		}
		k := pkgvoice.MergeKeys(cfg)
		if strings.TrimSpace(k.ElevenLabs) == "" {
			return nil, fmt.Errorf("voice: elevenlabs api key is empty")
		}
	case "cartesia":
		if strings.TrimSpace(os.Getenv("CARTESIA_VOICE_ID")) == "" {
			return nil, fmt.Errorf("voice: CARTESIA_VOICE_ID is required for cartesia tts")
		}
		k := pkgvoice.MergeKeys(cfg)
		if strings.TrimSpace(k.Cartesia) == "" {
			return nil, fmt.Errorf("voice: cartesia api key is empty")
		}
	default:
		return nil, fmt.Errorf("voice: unknown ttsProvider %q", vc.TTSProvider)
	}
	return nil, fmt.Errorf("voice: tts provider %q failed to initialize", want)
}
