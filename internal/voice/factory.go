package voice

import (
	"fmt"
	"os"
	"strings"

	"github.com/ageneralai/maven/internal/config"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

func normalizeSTT(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "deepgram"
	}
	return p
}

func normalizeTTS(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "openai"
	}
	return p
}

// NewSTT builds an STT implementation from channel voice settings.
func NewSTT(cfg config.VoiceConfig, k Keys) (pkgvoice.STT, error) {
	switch normalizeSTT(cfg.STTProvider) {
	case "deepgram":
		return &pkgvoice.DeepgramSTT{APIKey: k.Deepgram}, nil
	case "openai":
		return nil, fmt.Errorf("voice: sttProvider %q is not supported yet; use %q", cfg.STTProvider, "deepgram")
	default:
		return nil, fmt.Errorf("voice: unknown sttProvider %q", cfg.STTProvider)
	}
}

// NewTTS builds a TTS implementation from channel voice settings.
func NewTTS(cfg config.VoiceConfig, k Keys) (pkgvoice.TTS, error) {
	switch normalizeTTS(cfg.TTSProvider) {
	case "deepgram":
		return &pkgvoice.DeepgramTTS{APIKey: k.Deepgram}, nil
	case "openai":
		return &pkgvoice.OpenAITTS{APIKey: k.OpenAI}, nil
	case "elevenlabs":
		voiceID := strings.TrimSpace(os.Getenv("ELEVENLABS_VOICE_ID"))
		if voiceID == "" {
			return nil, fmt.Errorf("voice: ELEVENLABS_VOICE_ID is required for elevenlabs tts")
		}
		return &pkgvoice.ElevenLabsTTS{APIKey: k.ElevenLabs, VoiceID: voiceID}, nil
	case "cartesia":
		voiceID := strings.TrimSpace(os.Getenv("CARTESIA_VOICE_ID"))
		if voiceID == "" {
			return nil, fmt.Errorf("voice: CARTESIA_VOICE_ID is required for cartesia tts")
		}
		cts := &pkgvoice.CartesiaTTS{APIKey: k.Cartesia, VoiceID: voiceID}
		if m := strings.TrimSpace(os.Getenv("CARTESIA_MODEL_ID")); m != "" {
			cts.ModelID = m
		}
		if v := strings.TrimSpace(os.Getenv("CARTESIA_API_VERSION")); v != "" {
			cts.Version = v
		}
		return cts, nil
	default:
		return nil, fmt.Errorf("voice: unknown ttsProvider %q", cfg.TTSProvider)
	}
}
