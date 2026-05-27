package plugin

import (
	"context"
	"fmt"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/voice"
)

// Registry holds gateway plugins in registration order. Tools aggregates enabled plugins in that order.
type Registry struct {
	plugins []Plugin
}

// NewRegistry copies plugins into registration order for gateway wiring.
func NewRegistry(plugins ...Plugin) *Registry {
	cp := make([]Plugin, len(plugins))
	copy(cp, plugins)
	return &Registry{plugins: cp}
}

// Tools aggregates enabled plugin tools in registration order.
func (r *Registry) Tools(cfg *config.Config) []tool.Tool {
	if r == nil || cfg == nil {
		return nil
	}
	var out []tool.Tool
	for _, p := range r.plugins {
		if p.Enabled(cfg) {
			out = append(out, p.Tools(cfg)...)
		}
	}
	return out
}

// TTSProvider returns the first enabled plugin TTS implementation, if any.
func (r *Registry) TTSProvider(cfg *config.Config) voice.TTSProvider {
	if r == nil || cfg == nil {
		return nil
	}
	for _, p := range r.plugins {
		if !p.Enabled(cfg) {
			continue
		}
		if tts := p.TTSProvider(cfg); tts != nil {
			return tts
		}
	}
	return nil
}

// STTProvider returns the first enabled plugin STT implementation, if any.
func (r *Registry) STTProvider(cfg *config.Config) voice.STTProvider {
	if r == nil || cfg == nil {
		return nil
	}
	for _, p := range r.plugins {
		if !p.Enabled(cfg) {
			continue
		}
		if stt := p.STTProvider(cfg); stt != nil {
			return stt
		}
	}
	return nil
}

// Start runs plugin Start hooks in registration order. Fail-fast: the first error aborts and is returned
// so the gateway does not come up partially (same discipline as ChannelManager.startAll, which returns on the first error).
func (r *Registry) Start(ctx context.Context) error {
	if r == nil {
		return nil
	}
	for _, p := range r.plugins {
		if err := p.Start(ctx); err != nil {
			return fmt.Errorf("plugin %q start: %w", p.Name(), err)
		}
	}
	return nil
}

// Stop runs plugin Stop hooks in registration order. Fail-fast: the first error aborts and is returned.
func (r *Registry) Stop() error {
	if r == nil {
		return nil
	}
	for _, p := range r.plugins {
		if err := p.Stop(); err != nil {
			return fmt.Errorf("plugin %q stop: %w", p.Name(), err)
		}
	}
	return nil
}
