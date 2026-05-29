package deepgram

import (
	"context"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/httpc"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	pkgvoice "github.com/ageneralai/maven/internal/kernel/voice"
)

type Plugin struct{}

func NewPlugin() Plugin { return Plugin{} }

func (Plugin) Name() string { return "deepgram" }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "deepgram") {
		return nil
	}
	apiKey := strings.TrimSpace(cfg.Speech.Deepgram.APIKey)
	if apiKey == "" {
		return nil
	}
	httpClient, err := httpc.ClientFromProxy(cfg.Speech.Deepgram.Proxy)
	if err != nil {
		return nil
	}
	return &TTS{APIKey: apiKey, HTTPClient: httpClient}
}

func (Plugin) STTProvider(cfg *config.Config) pkgvoice.STTProvider {
	if cfg == nil || !pkgvoice.SelectedForSTT(cfg, "deepgram") {
		return nil
	}
	apiKey := strings.TrimSpace(cfg.Speech.Deepgram.APIKey)
	if apiKey == "" {
		return nil
	}
	httpClient, err := httpc.ClientFromProxy(cfg.Speech.Deepgram.Proxy)
	if err != nil {
		return nil
	}
	return &STT{APIKey: apiKey, HTTPClient: httpClient}
}

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }

var _ plugin.TTSPlugin = Plugin{}
var _ plugin.STTPlugin = Plugin{}
