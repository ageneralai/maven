package manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/ageneralai/maven/internal/bus"
	chann "github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/channel/feishu"
	"github.com/ageneralai/maven/internal/channel/matrix"
	"github.com/ageneralai/maven/internal/channel/telegram"
	"github.com/ageneralai/maven/internal/channel/web"
	"github.com/ageneralai/maven/internal/channel/wecom"
	"github.com/ageneralai/maven/internal/channel/whatsapp"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/plugin"
	mavenlog "github.com/ageneralai/maven/pkg/log"
)

type ChannelManager struct {
	mu       sync.RWMutex
	channels map[string]chann.Channel
	bus      *bus.MessageBus
	log      mavenlog.PrintLogger
	plugins  *plugin.Registry
	runner   web.StreamRunner
}

func NewChannelManager(b *bus.MessageBus, lg mavenlog.PrintLogger, plugins *plugin.Registry, runner web.StreamRunner) *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]chann.Channel),
		bus:      b,
		log:      lg,
		plugins:  plugins,
		runner:   runner,
	}
}

func buildChannelMap(cfg *config.Config, b *bus.MessageBus, lg mavenlog.PrintLogger, plugins *plugin.Registry, runner web.StreamRunner) (map[string]chann.Channel, error) {
	out := make(map[string]chann.Channel)
	ws := cfg.Agent.Workspace
	chcfg := cfg.Channels
	if chcfg.Telegram.Enabled {
		ch, err := telegram.NewTelegramChannel(chcfg.Telegram, ws, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init telegram channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	if chcfg.Feishu.Enabled {
		ch, err := feishu.NewFeishuChannel(chcfg.Feishu, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init feishu channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	if chcfg.WeCom.Enabled {
		ch, err := wecom.NewWeComChannel(chcfg.WeCom, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init wecom channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	if chcfg.WhatsApp.Enabled {
		ch, err := whatsapp.NewWhatsApp(chcfg.WhatsApp, lg, b)
		if err != nil {
			return nil, fmt.Errorf("create whatsapp channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	if chcfg.Matrix.Enabled {
		ch, err := matrix.NewMatrixChannel(chcfg.Matrix, ws, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init matrix channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	if chcfg.Web.Enabled {
		ch, err := web.NewWebChannel(chcfg.Web, cfg.Gateway, cfg, plugins, lg, b, runner)
		if err != nil {
			return nil, fmt.Errorf("init web channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	return out, nil
}

func (m *ChannelManager) Apply(ctx context.Context, cfg *config.Config) error {
	m.mu.Lock()
	old := m.channels
	oldNames := make([]string, 0, len(old))
	for n := range old {
		oldNames = append(oldNames, n)
	}
	m.mu.Unlock()
	for _, n := range oldNames {
		if ch := old[n]; ch != nil {
			_ = ch.Stop()
		}
		m.bus.SetOutboundSubscriber(n, nil)
	}
	next, err := buildChannelMap(cfg, m.bus, m.log, m.plugins, m.runner)
	if err != nil {
		return err
	}
	for name, ch := range next {
		n := name
		c := ch
		m.bus.SetOutboundSubscriber(n, func(msg bus.OutboundMessage) {
			if err := c.Send(context.Background(), msg); err != nil {
				m.log.Printf("[channel-mgr] send to %s failed: %v", n, err)
			}
		})
	}
	m.mu.Lock()
	m.channels = next
	m.mu.Unlock()
	return m.startAll(ctx, next)
}

func (m *ChannelManager) startAll(ctx context.Context, byName map[string]chann.Channel) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(byName))
	for name, ch := range byName {
		wg.Add(1)
		go func(name string, ch chann.Channel) {
			defer wg.Done()
			m.log.Printf("[channel-mgr] starting %s", name)
			if err := ch.Start(ctx); err != nil {
				errCh <- fmt.Errorf("%s: %w", name, err)
			}
		}(name, ch)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		return err
	}
	return nil
}

func (m *ChannelManager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	snap := make(map[string]chann.Channel, len(m.channels))
	for k, v := range m.channels {
		snap[k] = v
	}
	m.mu.RUnlock()
	return m.startAll(ctx, snap)
}

func (m *ChannelManager) StopAll() error {
	m.mu.RLock()
	snap := make(map[string]chann.Channel, len(m.channels))
	names := make([]string, 0, len(m.channels))
	for n, ch := range m.channels {
		snap[n] = ch
		names = append(names, n)
	}
	m.mu.RUnlock()
	for _, name := range names {
		m.log.Printf("[channel-mgr] stopping %s", name)
		if ch := snap[name]; ch != nil {
			if err := ch.Stop(); err != nil {
				m.log.Printf("[channel-mgr] error stopping %s: %v", name, err)
			}
		}
		m.bus.SetOutboundSubscriber(name, nil)
	}
	m.mu.Lock()
	m.channels = make(map[string]chann.Channel)
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

func (m *ChannelManager) GetChannel(name string) chann.Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.channels[name]
}
