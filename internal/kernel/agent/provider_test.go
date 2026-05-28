package agent

import (
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/maven/internal/kernel/config"
)

func TestNewProviderForModel_usesOverrideWhenNonEmpty(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Provider: config.ProviderConfig{Type: "anthropic", APIKey: "k"},
		Agent:    config.AgentConfig{Model: "primary", MaxTokens: 100},
	}
	p := NewProviderForModel(cfg, "override")
	ap, ok := p.(*model.AnthropicProvider)
	if !ok {
		t.Fatalf("got %T, want *model.AnthropicProvider", p)
	}
	if ap.ModelName != "override" {
		t.Errorf("ModelName = %q, want override", ap.ModelName)
	}
	cfg.Provider.Type = "openai"
	p = NewProviderForModel(cfg, "override")
	op, ok := p.(*model.OpenAIProvider)
	if !ok {
		t.Fatalf("got %T, want *model.OpenAIProvider", p)
	}
	if op.ModelName != "override" {
		t.Errorf("ModelName = %q, want override", op.ModelName)
	}
}

func TestNewProviderForModel_fallsBackToAgentModelWhenEmpty(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Provider: config.ProviderConfig{Type: "anthropic", APIKey: "k"},
		Agent:    config.AgentConfig{Model: "primary", MaxTokens: 100},
	}
	for _, override := range []string{"", "   ", "\t"} {
		t.Run(override, func(t *testing.T) {
			t.Parallel()
			p := NewProviderForModel(cfg, override)
			ap, ok := p.(*model.AnthropicProvider)
			if !ok {
				t.Fatalf("got %T, want *model.AnthropicProvider", p)
			}
			if ap.ModelName != "primary" {
				t.Errorf("ModelName = %q, want primary", ap.ModelName)
			}
		})
	}
}
