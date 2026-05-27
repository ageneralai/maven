package voice

import (
	"os"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/config"
)

// Keys holds provider credentials resolved from environment (and optional OpenAI key from config).
type Keys struct {
	// Deepgram is the Deepgram API key.
	Deepgram string
	// OpenAI is the OpenAI API key used for TTS when selected.
	OpenAI string
	// ElevenLabs is the ElevenLabs API key.
	ElevenLabs string
	// Cartesia is the Cartesia API key.
	Cartesia string
}

func firstNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}

// MergeKeys reads standard env vars and falls back to provider API key for OpenAI compatibility.
func MergeKeys(cfg *config.Config) Keys {
	k := Keys{
		Deepgram:   firstNonEmpty(os.Getenv("MAVEN_DEEPGRAM_API_KEY"), os.Getenv("DEEPGRAM_API_KEY")),
		ElevenLabs: firstNonEmpty(os.Getenv("MAVEN_ELEVENLABS_API_KEY"), os.Getenv("ELEVENLABS_API_KEY")),
		OpenAI:     firstNonEmpty(os.Getenv("OPENAI_API_KEY"), os.Getenv("MAVEN_OPENAI_API_KEY")),
		Cartesia:   firstNonEmpty(os.Getenv("MAVEN_CARTESIA_API_KEY"), os.Getenv("CARTESIA_API_KEY")),
	}
	if cfg != nil && strings.TrimSpace(k.OpenAI) == "" {
		k.OpenAI = strings.TrimSpace(cfg.Provider.APIKey)
	}
	return k
}
