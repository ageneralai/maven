package plugin

import (
	"context"
	"fmt"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
)

// Registry holds gateway plugins in registration order. Tools aggregates enabled plugins in that order.
type Registry struct {
	plugins []Plugin
}

func NewRegistry(plugins ...Plugin) *Registry {
	cp := make([]Plugin, len(plugins))
	copy(cp, plugins)
	return &Registry{plugins: cp}
}

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

func (r *Registry) Channels(cfg *config.Config) []channel.Channel {
	if r == nil || cfg == nil {
		return nil
	}
	var out []channel.Channel
	for _, p := range r.plugins {
		if !p.Enabled(cfg) {
			continue
		}
		chs := p.Channels(cfg)
		if chs != nil {
			out = append(out, chs...)
		}
	}
	return out
}

func (r *Registry) Provider(cfg *config.Config) api.ModelFactory {
	if r == nil || cfg == nil {
		return nil
	}
	for _, p := range r.plugins {
		if !p.Enabled(cfg) {
			continue
		}
		if prov := p.Provider(cfg); prov != nil {
			return prov
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
