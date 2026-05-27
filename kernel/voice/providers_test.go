package voice

import (
	"testing"

	"github.com/ageneralai/maven/kernel/config"
)

func TestSTTName_DefaultDeepgram(t *testing.T) {
	t.Parallel()
	if got := STTName(&config.Config{}); got != "deepgram" {
		t.Fatalf("STTName({}) = %q, want deepgram", got)
	}
}

func TestTTSName_DefaultOpenAI(t *testing.T) {
	t.Parallel()
	if got := TTSName(&config.Config{}); got != "openai" {
		t.Fatalf("TTSName({}) = %q, want openai", got)
	}
}

func TestSelectedForSTT(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Speech: config.SpeechConfig{STTProvider: "deepgram"}}
	if !SelectedForSTT(cfg, "deepgram") {
		t.Fatal("expected deepgram stt selected")
	}
	if SelectedForSTT(cfg, "openai") {
		t.Fatal("openai should not be selected for stt")
	}
}

func TestSelectedForTTS(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Speech: config.SpeechConfig{TTSProvider: "cartesia"}}
	if !SelectedForTTS(cfg, "cartesia") {
		t.Fatal("expected cartesia tts selected")
	}
}
