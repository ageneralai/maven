package voice

import "github.com/ageneralai/maven/kernel/config"

// STTName returns the configured speech-to-text provider name (default deepgram).
func STTName(cfg *config.Config) string {
	if cfg == nil {
		return NormalizeSTT("")
	}
	return NormalizeSTT(cfg.Speech.STTProvider)
}

// TTSName returns the configured text-to-speech provider name (default openai).
func TTSName(cfg *config.Config) string {
	if cfg == nil {
		return NormalizeTTS("")
	}
	return NormalizeTTS(cfg.Speech.TTSProvider)
}

// SelectedForSTT reports whether name is the active STT provider for cfg.
func SelectedForSTT(cfg *config.Config, name string) bool {
	return STTName(cfg) == name
}

// SelectedForTTS reports whether name is the active TTS provider for cfg.
func SelectedForTTS(cfg *config.Config, name string) bool {
	return TTSName(cfg) == name
}
