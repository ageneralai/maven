package web

import (
	"context"

	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/channels"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"log/slog"
)

type Plugin struct {
	bus     *bus.MessageBus
	log     *slog.Logger
	plugins *plugin.Registry
	runner  StreamRunner
}

func NewPlugin(b *bus.MessageBus, lg *slog.Logger) *Plugin {
	return &Plugin{bus: b, log: lg}
}

func (p *Plugin) SetRegistry(reg *plugin.Registry) { p.plugins = reg }

func (p *Plugin) SetStreamRunner(r StreamRunner) { p.runner = r }

func (p *Plugin) Name() string { return "web" }

func (p *Plugin) Start(context.Context) error { return nil }

func (p *Plugin) Stop() error { return nil }

func (p *Plugin) Channels(cfg *config.Config) []channels.Channel {
	if cfg == nil || !cfg.Channels.Web.Enabled {
		return nil
	}
	ch, err := NewWebChannel(cfg.Channels.Web, cfg.Gateway, cfg, p.plugins, p.log, p.bus, p.runner)
	if err != nil {
		p.log.Error("web channel init", "err", err)
		return nil
	}
	return []channels.Channel{ch}
}
