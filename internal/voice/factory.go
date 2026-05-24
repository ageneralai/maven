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

// NewSTT builds STT from speech config and plugin registry (gateway registry or DefaultVoiceRegistry).
func NewSTT(cfg *config.Config, reg *plugin.Registry) (pkgvoice.STT, error) {
	if cfg == nil {
		return nil, fmt.Errorf("speech: nil config")
	}
	reg = resolveRegistry(reg)
	if stt := reg.STTProvider(cfg); stt != nil {
		return stt, nil
	}
	want := pkgvoice.STTName(cfg)
	switch want {
	case "openai":
		return nil, fmt.Errorf("speech: sttProvider %q is not supported yet; use %q", cfg.Speech.STTProvider, "deepgram")
	case "deepgram":
		k := pkgvoice.MergeKeys(cfg)
		if strings.TrimSpace(k.Deepgram) == "" {
			return nil, fmt.Errorf("speech: deepgram api key is empty")
		}
	default:
		return nil, fmt.Errorf("speech: unknown sttProvider %q", cfg.Speech.STTProvider)
	}
	return nil, fmt.Errorf("speech: stt provider %q failed to initialize", want)
}

// NewTTS builds TTS from speech config and plugin registry.
func NewTTS(cfg *config.Config, reg *plugin.Registry) (pkgvoice.TTS, error) {
	if cfg == nil {
		return nil, fmt.Errorf("speech: nil config")
	}
	reg = resolveRegistry(reg)
	if tts := reg.TTSProvider(cfg); tts != nil {
		return tts, nil
	}
	want := pkgvoice.TTSName(cfg)
	switch want {
	case "deepgram":
		k := pkgvoice.MergeKeys(cfg)
		if strings.TrimSpace(k.Deepgram) == "" {
			return nil, fmt.Errorf("speech: deepgram api key is empty")
		}
	case "openai":
		k := pkgvoice.MergeKeys(cfg)
		if strings.TrimSpace(k.OpenAI) == "" {
			return nil, fmt.Errorf("speech: openai api key is empty")
		}
	case "elevenlabs":
		if strings.TrimSpace(os.Getenv("ELEVENLABS_VOICE_ID")) == "" {
			return nil, fmt.Errorf("speech: ELEVENLABS_VOICE_ID is required for elevenlabs tts")
		}
		k := pkgvoice.MergeKeys(cfg)
		if strings.TrimSpace(k.ElevenLabs) == "" {
			return nil, fmt.Errorf("speech: elevenlabs api key is empty")
		}
	case "cartesia":
		if strings.TrimSpace(os.Getenv("CARTESIA_VOICE_ID")) == "" {
			return nil, fmt.Errorf("speech: CARTESIA_VOICE_ID is required for cartesia tts")
		}
		k := pkgvoice.MergeKeys(cfg)
		if strings.TrimSpace(k.Cartesia) == "" {
			return nil, fmt.Errorf("speech: cartesia api key is empty")
		}
	default:
		return nil, fmt.Errorf("speech: unknown ttsProvider %q", cfg.Speech.TTSProvider)
	}
	return nil, fmt.Errorf("speech: tts provider %q failed to initialize", want)
}
