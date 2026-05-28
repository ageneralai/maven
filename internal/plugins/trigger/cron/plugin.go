package cron

import (
	"context"
	"sync"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/channel/manager"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/executor"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"log/slog"
)

// Plugin owns the cron scheduler, agent tools, slash commands, and background trigger.
type Plugin struct {
	storePath       string
	maxConcurrent   int
	channels        *manager.ChannelManager
	log             *slog.Logger
	mu              sync.RWMutex
	svc             *Service
}

// NewPlugin wires cron with store path and channel lookup for delivery. Service is created on trigger Start.
func NewPlugin(storePath string, maxConcurrent int, channels *manager.ChannelManager, lg *slog.Logger) *Plugin {
	return &Plugin{storePath: storePath, maxConcurrent: maxConcurrent, channels: channels, log: lg}
}

// Service returns the active scheduler after the trigger has started.
func (p *Plugin) Service() *Service {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.svc
}

// ServiceFromRegistry returns the cron Service from a registered cron Plugin, if present.
func ServiceFromRegistry(reg *plugin.Registry) *Service {
	if reg == nil {
		return nil
	}
	if cp, ok := reg.FindByName("cron").(*Plugin); ok {
		return cp.Service()
	}
	return nil
}

func (p *Plugin) Name() string { return "cron" }

func (p *Plugin) Start(context.Context) error { return nil }

func (p *Plugin) Stop() error { return nil }

func (p *Plugin) Triggers(*config.Config) []plugin.Trigger {
	return []plugin.Trigger{&runner{p: p}}
}

func (p *Plugin) Tools(*config.Config) []tool.Tool {
	return Tools(p, p.log)
}

func (p *Plugin) SlashCommands(*config.Config) []plugin.SlashCommand {
	p.mu.RLock()
	svc := p.svc
	p.mu.RUnlock()
	return slashCommands(svc)
}

// EnsureService creates the scheduler for tool/slash registration before the trigger loop runs.
func (p *Plugin) EnsureService(exec executor.TurnExecutor) (*Service, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.svc != nil {
		return p.svc, nil
	}
	deliver := &Deliver{Channels: p.channels, Log: p.log}
	svc, err := NewService(p.storePath, exec, p.maxConcurrent, p.log, deliver)
	if err != nil {
		return nil, err
	}
	p.svc = svc
	return svc, nil
}

func (p *Plugin) startLoop(ctx context.Context, exec executor.TurnExecutor, deliver *Deliver) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.svc != nil {
		p.svc.Stop()
		p.svc = nil
	}
	svc, err := NewService(p.storePath, exec, p.maxConcurrent, p.log, deliver)
	if err != nil {
		return err
	}
	p.svc = svc
	return svc.Start(ctx)
}

func (p *Plugin) stopLoop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.svc != nil {
		p.svc.Stop()
		p.svc = nil
	}
}
