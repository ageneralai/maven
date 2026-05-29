package manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/channel"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"log/slog"

	"golang.org/x/sync/errgroup"
)

type ChannelManager struct {
	mu              sync.RWMutex
	channels        map[string]channels.Channel
	bus             *bus.MessageBus
	log             *slog.Logger
	registry        *plugin.Registry
	pipelineSlashes []channels.PipelineSlashDefinition
}

func New(b *bus.MessageBus, lg *slog.Logger, reg *plugin.Registry) *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]channels.Channel),
		bus:      b,
		log:      lg,
		registry: reg,
	}
}

func (m *ChannelManager) SetRegistry(reg *plugin.Registry) {
	m.registry = reg
}

func (m *ChannelManager) SetPipelineSlashCommands(defs []channels.PipelineSlashDefinition) {
	m.pipelineSlashes = defs
}

func (m *ChannelManager) channelsFromRegistry(cfg *config.Config) (map[string]channels.Channel, error) {
	if m.registry == nil {
		return nil, fmt.Errorf("channel manager: nil registry")
	}
	out := make(map[string]channels.Channel)
	for _, ch := range m.registry.Channels(cfg) {
		if ch == nil {
			continue
		}
		name := ch.Name()
		if name == "" {
			return nil, fmt.Errorf("channel with empty name")
		}
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("duplicate channel %q", name)
		}
		out[name] = ch
	}
	return out, nil
}

func (m *ChannelManager) Apply(ctx context.Context, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if err := cfg.Channels.Validate(); err != nil {
		return err
	}
	next, err := m.channelsFromRegistry(cfg)
	if err != nil {
		return err
	}
	m.mu.Lock()
	old := m.channels
	m.mu.Unlock()
	oldNames := make([]string, 0, len(old))
	for n := range old {
		oldNames = append(oldNames, n)
	}
	for _, n := range oldNames {
		if ch := old[n]; ch != nil {
			_ = ch.Stop()
		}
		m.bus.SetOutboundSubscriber(n, nil)
	}
	for name, ch := range next {
		n := name
		c := ch
		m.bus.SetOutboundSubscriber(n, func(msg bus.OutboundMessage) error {
			if err := c.Send(ctx, msg); err != nil {
				m.log.Error("channel send failed", "channel", n, "err", err)
				return channels.WrapDeliveryFailed(err)
			}
			return nil
		})
	}
	m.mu.Lock()
	m.channels = next
	m.mu.Unlock()
	for _, ch := range next {
		if cfg, ok := ch.(channels.PipelineSlashConfigurer); ok {
			cfg.SetPipelineSlashCommands(m.pipelineSlashes)
		}
	}
	return m.startAll(ctx, next)
}

func (m *ChannelManager) startAll(ctx context.Context, byName map[string]channels.Channel) error {
	var g errgroup.Group
	for name, ch := range byName {
		n, c := name, ch
		g.Go(func() error {
			m.log.Info("starting channel", "channel", n)
			if err := c.Start(ctx); err != nil {
				return fmt.Errorf("%s: %w", n, err)
			}
			return nil
		})
	}
	return g.Wait()
}

func (m *ChannelManager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	snap := make(map[string]channels.Channel, len(m.channels))
	for k, v := range m.channels {
		snap[k] = v
	}
	m.mu.RUnlock()
	return m.startAll(ctx, snap)
}

func (m *ChannelManager) StopAll() error {
	m.mu.RLock()
	snap := make(map[string]channels.Channel, len(m.channels))
	names := make([]string, 0, len(m.channels))
	for n, ch := range m.channels {
		snap[n] = ch
		names = append(names, n)
	}
	m.mu.RUnlock()
	for _, name := range names {
		m.log.Info("stopping channel", "channel", name)
		if ch := snap[name]; ch != nil {
			if err := ch.Stop(); err != nil {
				m.log.Error("channel stop error", "channel", name, "err", err)
			}
		}
		m.bus.SetOutboundSubscriber(name, nil)
	}
	m.mu.Lock()
	m.channels = make(map[string]channels.Channel)
	m.mu.Unlock()
	return nil
}

func (m *ChannelManager) EnabledChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	return names
}

func (m *ChannelManager) GetChannel(name string) channels.Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.channels[name]
}
