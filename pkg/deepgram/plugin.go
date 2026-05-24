package deepgram

import (
	"context"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/plugin"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

// Plugin exposes Deepgram STT/TTS when speech config selects deepgram.
type Plugin struct{}

func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "deepgram" }

func (Plugin) Enabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.Deepgram) == "" {
		return false
	}
	return pkgvoice.SelectedForSTT(cfg, "deepgram") || pkgvoice.SelectedForTTS(cfg, "deepgram")
}

func (Plugin) Tools(*config.Config) []tool.Tool { return nil }

func (Plugin) Channels(*config.Config) []channel.Channel { return nil }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "deepgram") {
		return nil
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.Deepgram) == "" {
		return nil
	}
	return &TTS{APIKey: k.Deepgram}
}

func (Plugin) STTProvider(cfg *config.Config) pkgvoice.STTProvider {
	if cfg == nil || !pkgvoice.SelectedForSTT(cfg, "deepgram") {
		return nil
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.Deepgram) == "" {
		return nil
	}
	return &STT{APIKey: k.Deepgram}
}

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
