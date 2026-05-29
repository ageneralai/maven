package feishu

import (
	"context"

	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/channels"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"log/slog"
)

type Plugin struct {
	bus *bus.MessageBus
	log *slog.Logger
}

func NewPlugin(b *bus.MessageBus, lg *slog.Logger) *Plugin {
	return &Plugin{bus: b, log: lg}
}

func (p *Plugin) Name() string { return "feishu" }

func (p *Plugin) Start(context.Context) error { return nil }

func (p *Plugin) Stop() error { return nil }

func (p *Plugin) Channels(cfg *config.Config) []channels.Channel {
	if cfg == nil || !cfg.Channels.Feishu.Enabled {
		return nil
	}
	ch, err := NewFeishuChannel(cfg.Channels.Feishu, p.log, p.bus)
	if err != nil {
		p.log.Error("feishu channel init", "err", err)
		return nil
	}
	return []channels.Channel{ch}
}

var _ plugin.ChannelPlugin = (*Plugin)(nil)
