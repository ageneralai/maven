package voice

import "strings"

// NormalizeSTT returns the canonical stt provider name for config (default deepgram).
func NormalizeSTT(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "deepgram"
	}
	return p
}

// NormalizeTTS returns the canonical tts provider name for config (default openai).
func NormalizeTTS(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "openai"
	}
	return p
}
