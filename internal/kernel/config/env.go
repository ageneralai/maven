package config

import (
	"os"
	"strings"
)

func applyEnv(cfg *Config) {
	switch cfg.Provider.Type {
	case "openai":
		setFromEnv(&cfg.Provider.APIKey, os.Getenv("OPENAI_API_KEY"))
	default:
		setFromEnv(&cfg.Provider.APIKey, os.Getenv("ANTHROPIC_API_KEY"))
	}
	setFromEnv(&cfg.Speech.OpenAI.APIKey, os.Getenv("OPENAI_API_KEY"))
	setFromEnv(&cfg.Speech.Deepgram.APIKey, os.Getenv("DEEPGRAM_API_KEY"))
	setFromEnv(&cfg.Speech.ElevenLabs.APIKey, os.Getenv("ELEVENLABS_API_KEY"))
	setFromEnv(&cfg.Speech.ElevenLabs.VoiceID, os.Getenv("ELEVENLABS_VOICE_ID"))
	setFromEnv(&cfg.Speech.Cartesia.APIKey, os.Getenv("CARTESIA_API_KEY"))
	setFromEnv(&cfg.Speech.Cartesia.VoiceID, os.Getenv("CARTESIA_VOICE_ID"))
	setFromEnv(&cfg.Logging.Level, os.Getenv("MAVEN_LOG_LEVEL"))
}

func setFromEnv(field *string, val string) {
	if v := strings.TrimSpace(val); v != "" {
		*field = v
	}
}
