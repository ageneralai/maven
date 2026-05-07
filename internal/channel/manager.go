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
	channels map[string]Channel
	bus      *bus.MessageBus
	log      mavenlog.PrintLogger
}

func NewChannelManager(cfg config.ChannelsConfig, agentWorkspace string, b *bus.MessageBus, lg mavenlog.PrintLogger) (*ChannelManager, error) {
	m := &ChannelManager{
		channels: make(map[string]Channel),
		bus:      b,
		log:      lg,
	}

	if cfg.Telegram.Enabled {
		ch, err := NewTelegramChannel(cfg.Telegram, agentWorkspace, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init telegram channel: %w", err)
		}
		m.channels[ch.Name()] = ch
		b.SubscribeOutbound(ch.Name(), func(msg bus.OutboundMessage) {
			if err := ch.Send(context.Background(), msg); err != nil {
				lg.Printf("[channel-mgr] send to %s failed: %v", ch.Name(), err)
			}
		})
	}

	if cfg.Feishu.Enabled {
		ch, err := NewFeishuChannel(cfg.Feishu, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init feishu channel: %w", err)
		}
		m.channels[ch.Name()] = ch
		b.SubscribeOutbound(ch.Name(), func(msg bus.OutboundMessage) {
			if err := ch.Send(context.Background(), msg); err != nil {
				lg.Printf("[channel-mgr] send to %s failed: %v", ch.Name(), err)
			}
		})
	}

	if cfg.WeCom.Enabled {
		ch, err := NewWeComChannel(cfg.WeCom, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init wecom channel: %w", err)
		}
		m.channels[ch.Name()] = ch
		b.SubscribeOutbound(ch.Name(), func(msg bus.OutboundMessage) {
			if err := ch.Send(context.Background(), msg); err != nil {
				lg.Printf("[channel-mgr] send to %s failed: %v", ch.Name(), err)
			}
		})
	}

	if cfg.WhatsApp.Enabled {
		ch, err := NewWhatsApp(cfg.WhatsApp, lg, b)
		if err != nil {
			return nil, fmt.Errorf("create whatsapp channel: %w", err)
		}
		m.channels[ch.Name()] = ch
		b.SubscribeOutbound(ch.Name(), func(msg bus.OutboundMessage) {
			if err := ch.Send(context.Background(), msg); err != nil {
				lg.Printf("[channel-mgr] send to %s failed: %v", ch.Name(), err)
			}
		})
	}

	return m, nil
}

func NewChannelManagerWithGateway(cfg config.ChannelsConfig, gwCfg config.GatewayConfig, agentWorkspace string, b *bus.MessageBus, lg mavenlog.PrintLogger) (*ChannelManager, error) {
	m, err := NewChannelManager(cfg, agentWorkspace, b, lg)
	if err != nil {
		return nil, err
	}

	if cfg.WebUI.Enabled {
		ch, err := NewWebUIChannel(cfg.WebUI, gwCfg, lg, b)
		if err != nil {
			return nil, fmt.Errorf("init webui channel: %w", err)
		}
		m.channels[ch.Name()] = ch
		b.SubscribeOutbound(ch.Name(), func(msg bus.OutboundMessage) {
			if err := ch.Send(context.Background(), msg); err != nil {
				lg.Printf("[channel-mgr] send to %s failed: %v", ch.Name(), err)
			}
		})
	}

	return m, nil
}

func (m *ChannelManager) StartAll(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(m.channels))

	for name, ch := range m.channels {
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

func (m *ChannelManager) StopAll() error {
	for name, ch := range m.channels {
		m.log.Printf("[channel-mgr] stopping %s", name)
		if err := ch.Stop(); err != nil {
			m.log.Printf("[channel-mgr] error stopping %s: %v", name, err)
		}
	}
	return nil
}

func (m *ChannelManager) EnabledChannels() []string {
	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	return names
}

// GetChannel returns a channel by name, or nil if not found.
func (m *ChannelManager) GetChannel(name string) Channel {
	return m.channels[name]
}
