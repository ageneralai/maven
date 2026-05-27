package openai

import (
	"context"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/httpc"
	"github.com/ageneralai/maven/pkg/plugin"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

// Plugin exposes OpenAI TTS when speech config selects openai.
type Plugin struct{}

func NewPlugin() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "openai" }

func (Plugin) Enabled(cfg *config.Config) bool {
	if cfg == nil || !pkgvoice.SelectedForTTS(cfg, "openai") {
		return false
	}
	k := pkgvoice.MergeKeys(cfg)
	if strings.TrimSpace(k.OpenAI) == "" {
		return false
	}
	_, err := httpc.ClientFromProxy(cfg.Speech.OpenAI.Proxy)
	return err == nil
}

func (Plugin) Tools(*config.Config) []tool.Tool { return nil }

func (Plugin) Channels(*config.Config) []channel.Channel { return nil }

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

func (Plugin) STTProvider(*config.Config) pkgvoice.STTProvider { return nil }

func (Plugin) Start(context.Context) error { return nil }

func (Plugin) Stop() error { return nil }
