package channel

import (
	"context"
	"fmt"
	"sync"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/config"
	mavenlog "github.com/ageneralai/maven/internal/log"
)

type ChannelManager struct {
	mu       sync.RWMutex
	channels map[string]Channel
	bus      *bus.MessageBus
	log      mavenlog.PrintLogger
}

func NewChannelManager(b *bus.MessageBus, lg mavenlog.PrintLogger) *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]Channel),
		bus:      b,
		log:      lg,
	}
}

func buildChannelMap(cfg *config.Config, b *bus.MessageBus, lg mavenlog.PrintLogger) (map[string]Channel, error) {
	out := make(map[string]Channel)
	ws := cfg.Agent.Workspace
	chcfg := cfg.Channels
	if chcfg.Telegram.Enabled {
		ch, err := NewTelegramChannel(chcfg.Telegram, ws, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init telegram channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	if chcfg.Feishu.Enabled {
		ch, err := NewFeishuChannel(chcfg.Feishu, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init feishu channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	if chcfg.WeCom.Enabled {
		ch, err := NewWeComChannel(chcfg.WeCom, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init wecom channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	if chcfg.WhatsApp.Enabled {
		ch, err := NewWhatsApp(chcfg.WhatsApp, lg, b)
		if err != nil {
			return nil, fmt.Errorf("create whatsapp channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	if chcfg.WebUI.Enabled {
		ch, err := NewWebUIChannel(chcfg.WebUI, cfg.Gateway, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init webui channel: %w", err)
		}
		out[ch.Name()] = ch
	}
	return out, nil
}

// Apply stops and unsubscribes previous instances, constructs new ones,
// registers outbound subscribers, and starts them.
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
	next, err := buildChannelMap(cfg, m.bus, m.log)
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

func (m *ChannelManager) startAll(ctx context.Context, channels map[string]Channel) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(channels))
	for name, ch := range channels {
		wg.Add(1)
		go func(name string, ch Channel) {
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

// StartAll starts every registered channel (after Apply populated the map).
func (m *ChannelManager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	snap := make(map[string]Channel, len(m.channels))
	for k, v := range m.channels {
		snap[k] = v
	}
	m.mu.RUnlock()
	return m.startAll(ctx, snap)
}

func (m *ChannelManager) StopAll() error {
	m.mu.RLock()
	snap := make(map[string]Channel, len(m.channels))
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
	m.channels = make(map[string]Channel)
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

// GetChannel returns a channel by name, or nil if not found.
func (m *ChannelManager) GetChannel(name string) Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.channels[name]
}
