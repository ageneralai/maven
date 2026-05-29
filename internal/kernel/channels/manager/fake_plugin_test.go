package manager

import (
	"context"

	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/channel"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
)

type fakeFeishuPlugin struct {
	bus *bus.MessageBus
}

func (f *fakeFeishuPlugin) Name() string { return "feishu-test" }

func (f *fakeFeishuPlugin) Start(context.Context) error { return nil }

func (f *fakeFeishuPlugin) Stop() error { return nil }

func (f *fakeFeishuPlugin) Channels(cfg *config.Config) []channels.Channel {
	if cfg == nil || !cfg.Channels.Feishu.Enabled {
		return nil
	}
	if cfg.Channels.Feishu.AppID == "" || cfg.Channels.Feishu.AppSecret == "" {
		return nil
	}
	return []channels.Channel{&mockFeishuChannel{name: "feishu"}}
}

type mockFeishuChannel struct{ name string }

func (m *mockFeishuChannel) Name() string { return m.name }

func (m *mockFeishuChannel) Start(context.Context) error { return nil }

func (m *mockFeishuChannel) Stop() error { return nil }

func (m *mockFeishuChannel) Send(context.Context, bus.OutboundMessage) error { return nil }

func (m *mockFeishuChannel) Capabilities() channels.CapabilitySet { return channels.CapabilitySet{} }

func feishuTestRegistry(b *bus.MessageBus) *plugin.Registry {
	return plugin.NewRegistry(&fakeFeishuPlugin{bus: b})
}

type fakeWeComPlugin struct{ bus *bus.MessageBus }

func (f *fakeWeComPlugin) Name() string { return "wecom-test" }

func (f *fakeWeComPlugin) Start(context.Context) error { return nil }

func (f *fakeWeComPlugin) Stop() error { return nil }

func (f *fakeWeComPlugin) Channels(cfg *config.Config) []channels.Channel {
	if cfg == nil || !cfg.Channels.WeCom.Enabled {
		return nil
	}
	if cfg.Channels.WeCom.Token == "" || cfg.Channels.WeCom.EncodingAESKey == "" {
		return nil
	}
	return []channels.Channel{&mockFeishuChannel{name: "wecom"}}
}

func wecomTestRegistry(b *bus.MessageBus) *plugin.Registry {
	return plugin.NewRegistry(&fakeWeComPlugin{bus: b})
}
