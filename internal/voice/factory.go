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
	switch pkgvoice.STTName(cfg) {
	case "deepgram":
		if strings.TrimSpace(pkgvoice.MergeKeys(cfg).Deepgram) == "" {
			return nil, fmt.Errorf("speech: deepgram api key is empty")
		}
		return nil, fmt.Errorf("speech: deepgram stt provider not registered")
	case "openai":
		return nil, fmt.Errorf("speech: sttProvider %q not supported; use %q", cfg.Speech.STTProvider, "deepgram")
	default:
		return nil, fmt.Errorf("speech: unknown sttProvider %q", cfg.Speech.STTProvider)
	}
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
	switch pkgvoice.TTSName(cfg) {
	case "deepgram":
		if strings.TrimSpace(pkgvoice.MergeKeys(cfg).Deepgram) == "" {
			return nil, fmt.Errorf("speech: deepgram api key is empty")
		}
		return nil, fmt.Errorf("speech: deepgram tts provider not registered")
	case "openai":
		if strings.TrimSpace(pkgvoice.MergeKeys(cfg).OpenAI) == "" {
			return nil, fmt.Errorf("speech: openai api key is empty")
		}
		return nil, fmt.Errorf("speech: openai tts provider not registered")
	case "elevenlabs":
		if strings.TrimSpace(os.Getenv("ELEVENLABS_VOICE_ID")) == "" {
			return nil, fmt.Errorf("speech: ELEVENLABS_VOICE_ID is required for elevenlabs tts")
		}
		if strings.TrimSpace(pkgvoice.MergeKeys(cfg).ElevenLabs) == "" {
			return nil, fmt.Errorf("speech: elevenlabs api key is empty")
		}
		return nil, fmt.Errorf("speech: elevenlabs tts provider not registered")
	case "cartesia":
		if strings.TrimSpace(os.Getenv("CARTESIA_VOICE_ID")) == "" {
			return nil, fmt.Errorf("speech: CARTESIA_VOICE_ID is required for cartesia tts")
		}
		if strings.TrimSpace(pkgvoice.MergeKeys(cfg).Cartesia) == "" {
			return nil, fmt.Errorf("speech: cartesia api key is empty")
		}
		return nil, fmt.Errorf("speech: cartesia tts provider not registered")
	default:
		return nil, fmt.Errorf("speech: unknown ttsProvider %q", cfg.Speech.TTSProvider)
	}
}
