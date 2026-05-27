package matrix

import (
	"context"

	"github.com/ageneralai/maven/kernel/bus"
	"github.com/ageneralai/maven/kernel/channels"
	"github.com/ageneralai/maven/kernel/config"
	"github.com/ageneralai/maven/kernel/plugin"
	"log/slog"
)

type Plugin struct {
	bus *bus.MessageBus
	log *slog.Logger
}

func NewPlugin(b *bus.MessageBus, lg *slog.Logger) plugin.ChannelPlugin {
	return &Plugin{bus: b, log: lg}
}

func (p *Plugin) Name() string { return "matrix" }

func (p *Plugin) Start(context.Context) error { return nil }

func (p *Plugin) Stop() error { return nil }

func (p *Plugin) Channels(cfg *config.Config) []channels.Channel {
	if cfg == nil || !cfg.Channels.Matrix.Enabled {
		return nil
	}
	ch, err := NewMatrixChannel(cfg.Channels.Matrix, cfg.Agent.Workspace, p.log, p.bus)
	if err != nil {
		p.log.Error("matrix channel init", "err", err)
		return nil
	}
	return []channels.Channel{ch}
}
