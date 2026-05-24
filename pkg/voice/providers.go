package voice

import "github.com/ageneralai/maven/internal/config"

func STTName(cfg *config.Config) string {
	if cfg == nil {
		return NormalizeSTT("")
	}
	return NormalizeSTT(cfg.Speech.STTProvider)
}

func TTSName(cfg *config.Config) string {
	if cfg == nil {
		return NormalizeTTS("")
	}
	return NormalizeTTS(cfg.Speech.TTSProvider)
}

func SelectedForSTT(cfg *config.Config, name string) bool {
	return STTName(cfg) == name
}

func SelectedForTTS(cfg *config.Config, name string) bool {
	return TTSName(cfg) == name
}
