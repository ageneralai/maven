package feishu

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
