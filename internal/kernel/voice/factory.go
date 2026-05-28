package voice

import (
	"fmt"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/config"
)

// ProviderRegistry is the narrow interface factory needs from the plugin registry.
type ProviderRegistry interface {
	TTSProvider(cfg *config.Config) TTS
	STTProvider(cfg *config.Config) STT
}

// NewSTT builds STT from speech config and a provider registry.
func NewSTT(cfg *config.Config, reg ProviderRegistry) (STT, error) {
	if cfg == nil {
		return nil, fmt.Errorf("speech: nil config")
	}
	if reg != nil {
		if stt := reg.STTProvider(cfg); stt != nil {
			return stt, nil
		}
	}
	switch STTName(cfg) {
	case "deepgram":
		if strings.TrimSpace(cfg.Speech.Deepgram.APIKey) == "" {
			return nil, fmt.Errorf("speech: deepgram api key is empty")
		}
		return nil, fmt.Errorf("speech: deepgram stt provider not registered")
	case "openai":
		return nil, fmt.Errorf("speech: sttProvider %q not supported; use %q", cfg.Speech.STTProvider, "deepgram")
	default:
		return nil, fmt.Errorf("speech: unknown sttProvider %q", cfg.Speech.STTProvider)
	}
}

// NewTTS builds TTS from speech config and a provider registry.
func NewTTS(cfg *config.Config, reg ProviderRegistry) (TTS, error) {
	if cfg == nil {
		return nil, fmt.Errorf("speech: nil config")
	}
	if reg != nil {
		if tts := reg.TTSProvider(cfg); tts != nil {
			return tts, nil
		}
	}
	switch TTSName(cfg) {
	case "deepgram":
		if strings.TrimSpace(cfg.Speech.Deepgram.APIKey) == "" {
			return nil, fmt.Errorf("speech: deepgram api key is empty")
		}
		return nil, fmt.Errorf("speech: deepgram tts provider not registered")
	case "openai":
		apiKey := cfg.Speech.OpenAI.APIKey
		if apiKey == "" {
			apiKey = cfg.Provider.APIKey
		}
		if strings.TrimSpace(apiKey) == "" {
			return nil, fmt.Errorf("speech: openai api key is empty")
		}
		return nil, fmt.Errorf("speech: openai tts provider not registered")
	case "elevenlabs":
		if strings.TrimSpace(cfg.Speech.ElevenLabs.VoiceID) == "" {
			return nil, fmt.Errorf("speech: speech.elevenlabs.voiceId is required for elevenlabs tts")
		}
		if strings.TrimSpace(cfg.Speech.ElevenLabs.APIKey) == "" {
			return nil, fmt.Errorf("speech: elevenlabs api key is empty")
		}
		return nil, fmt.Errorf("speech: elevenlabs tts provider not registered")
	case "cartesia":
		if strings.TrimSpace(cfg.Speech.Cartesia.VoiceID) == "" {
			return nil, fmt.Errorf("speech: speech.cartesia.voiceId is required for cartesia tts")
		}
		if strings.TrimSpace(cfg.Speech.Cartesia.APIKey) == "" {
			return nil, fmt.Errorf("speech: cartesia api key is empty")
		}
		return nil, fmt.Errorf("speech: cartesia tts provider not registered")
	default:
		return nil, fmt.Errorf("speech: unknown ttsProvider %q", cfg.Speech.TTSProvider)
	}
}
