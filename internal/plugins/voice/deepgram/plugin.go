package deepgram

import (
	"context"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/ageneralai/maven/internal/kernel/httpc"
	pkgvoice "github.com/ageneralai/maven/internal/kernel/voice"
)

// Plugin exposes Deepgram STT/TTS when speech config selects deepgram.
type Plugin struct{}

func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "deepgram" }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "deepgram") {
		return nil
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.Deepgram) == "" {
		return nil
	}
	httpClient, err := httpc.ClientFromProxy(cfg.Speech.Deepgram.Proxy)
	if err != nil {
		return nil
	}
	return &TTS{APIKey: k.Deepgram, HTTPClient: httpClient}
}

func (Plugin) STTProvider(cfg *config.Config) pkgvoice.STTProvider {
	if cfg == nil || !pkgvoice.SelectedForSTT(cfg, "deepgram") {
		return nil
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.Deepgram) == "" {
		return nil
	}
	httpClient, err := httpc.ClientFromProxy(cfg.Speech.Deepgram.Proxy)
	if err != nil {
		return nil
	}
	return &STT{APIKey: k.Deepgram, HTTPClient: httpClient}
}

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
