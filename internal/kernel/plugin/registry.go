package plugin

import (
	"context"
	"fmt"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/channel"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/hook"
	"github.com/ageneralai/maven/internal/kernel/voice"
)

// Registry holds plugins in registration order.
type Registry struct {
	plugins []Plugin
}

// NewRegistry copies plugins into registration order for gateway wiring.
func NewRegistry(plugins ...Plugin) *Registry {
	cp := make([]Plugin, len(plugins))
	copy(cp, plugins)
	return &Registry{plugins: cp}
}

// FindByName returns the first plugin with the given Name(), or nil.
func (r *Registry) FindByName(name string) Plugin {
	if r == nil {
		return nil
	}
	for _, p := range r.plugins {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

func (r *Registry) Channels(cfg *config.Config) []channels.Channel {
	if r == nil || cfg == nil {
		return nil
	}
	var out []channels.Channel
	for _, p := range r.plugins {
		if cp, ok := p.(ChannelPlugin); ok {
			out = append(out, cp.Channels(cfg)...)
		}
	}
	return out
}

func (r *Registry) Tools(cfg *config.Config) []tool.Tool {
	if r == nil || cfg == nil {
		return nil
	}
	var out []tool.Tool
	for _, p := range r.plugins {
		if tp, ok := p.(ToolPlugin); ok {
			out = append(out, tp.Tools(cfg)...)
		}
	}
	return out
}

func (r *Registry) Skills(cfg *config.Config) []api.SkillRegistration {
	if r == nil || cfg == nil {
		return nil
	}
	var out []api.SkillRegistration
	for _, p := range r.plugins {
		if sp, ok := p.(SkillPlugin); ok {
			out = append(out, sp.Skills(cfg)...)
		}
	}
	return out
}

func (r *Registry) Triggers(cfg *config.Config) []Trigger {
	if r == nil || cfg == nil {
		return nil
	}
	var out []Trigger
	for _, p := range r.plugins {
		if tp, ok := p.(TriggerPlugin); ok {
			out = append(out, tp.Triggers(cfg)...)
		}
	}
	return out
}

func (r *Registry) SlashCommands(cfg *config.Config) []SlashCommand {
	if r == nil || cfg == nil {
		return nil
	}
	var out []SlashCommand
	for _, p := range r.plugins {
		if sp, ok := p.(SlashPlugin); ok {
			out = append(out, sp.SlashCommands(cfg)...)
		}
	}
	return out
}

func (r *Registry) TTSProvider(cfg *config.Config) voice.TTSProvider {
	if r == nil || cfg == nil {
		return nil
	}
	for _, p := range r.plugins {
		if tp, ok := p.(TTSPlugin); ok {
			if v := tp.TTSProvider(cfg); v != nil {
				return v
			}
		}
	}
	return nil
}

func (r *Registry) STTProvider(cfg *config.Config) voice.STTProvider {
	if r == nil || cfg == nil {
		return nil
	}
	for _, p := range r.plugins {
		if tp, ok := p.(STTPlugin); ok {
			if v := tp.STTProvider(cfg); v != nil {
				return v
			}
		}
	}
	return nil
}

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

// PostTurnHandlers returns one handler per PostTurnPlugin implementation, in registration order.
func (r *Registry) PostTurnHandlers(cfg *config.Config) []hook.PostTurnHandler {
	if r == nil || cfg == nil {
		return nil
	}
	var out []hook.PostTurnHandler
	for _, p := range r.plugins {
		if ptp, ok := p.(PostTurnPlugin); ok {
			if h := ptp.PostTurnHandler(cfg); h != nil {
				out = append(out, h)
			}
		}
	}
	return out
}

// MemoryPlugins returns all registered MemoryPlugin implementations in registration order.
func (r *Registry) MemoryPlugins() []MemoryPlugin {
	if r == nil {
		return nil
	}
	var out []MemoryPlugin
	for _, p := range r.plugins {
		if mp, ok := p.(MemoryPlugin); ok {
			out = append(out, mp)
		}
	}
	return out
}
