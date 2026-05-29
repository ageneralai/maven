package openai

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

func (Plugin) Name() string { return "openai" }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "openai") {
		return nil
	}
	apiKey := cfg.Speech.OpenAI.APIKey
	if apiKey == "" {
		apiKey = cfg.Provider.APIKey
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	httpClient, err := httpc.ClientFromProxy(cfg.Speech.OpenAI.Proxy)
	if err != nil {
		return nil
	}
	return &TTS{APIKey: apiKey, HTTPClient: httpClient}
}

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }

var _ plugin.TTSPlugin = Plugin{}
