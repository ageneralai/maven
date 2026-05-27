package openai

import (
	"context"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/ageneralai/maven/internal/kernel/httpc"
	pkgvoice "github.com/ageneralai/maven/internal/kernel/voice"
)

// Plugin exposes OpenAI TTS when speech config selects openai.
type Plugin struct{}

func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "openai" }

func (Plugin) TTSProvider(cfg *config.Config) pkgvoice.TTSProvider {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "openai") {
		return nil
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.OpenAI) == "" {
		return nil
	}
	httpClient, err := httpc.ClientFromProxy(cfg.Speech.OpenAI.Proxy)
	if err != nil {
		return nil
	}
	return &TTS{APIKey: k.OpenAI, HTTPClient: httpClient}
}

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
