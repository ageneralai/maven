package manager

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/config"
	"log/slog"
)

func TestChannelManager_FeishuEnabled(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	b := bus.New(10, log)
	reg := feishuTestRegistry(b)
	m := New(b, log, reg)
	cfg := &config.Config{
		Agent: config.AgentConfig{Workspace: t.TempDir()},
		Channels: config.ChannelsConfig{
			Feishu: config.FeishuConfig{
				Enabled:   true,
				AppID:     "cli_test",
				AppSecret: "secret",
			},
		},
		Gateway: config.GatewayConfig{Host: config.DefaultHost, Port: config.DefaultPort},
	}
	if err := m.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	names := m.EnabledChannels()
	if len(names) != 1 || names[0] != "feishu" {
		t.Errorf("EnabledChannels = %v, want [feishu]", names)
	}
}

func TestChannelManager_FeishuEnabled_MissingConfig(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	b := bus.New(10, log)
	reg := feishuTestRegistry(b)
	m := New(b, log, reg)
	cfg := &config.Config{
		Agent: config.AgentConfig{Workspace: t.TempDir()},
		Channels: config.ChannelsConfig{
			Feishu: config.FeishuConfig{Enabled: true},
		},
		Gateway: config.GatewayConfig{Host: config.DefaultHost, Port: config.DefaultPort},
	}
	if err := m.Apply(context.Background(), cfg); err == nil {
		t.Error("expected error for missing feishu config")
	}
}

func TestChannelManager_WeComEnabled_MissingConfig(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	b := bus.New(10, log)
	reg := wecomTestRegistry(b)
	m := New(b, log, reg)
	cfg := &config.Config{
		Agent: config.AgentConfig{Workspace: t.TempDir()},
		Channels: config.ChannelsConfig{
			WeCom: config.WeComConfig{Enabled: true},
		},
		Gateway: config.GatewayConfig{Host: config.DefaultHost, Port: config.DefaultPort},
	}
	if err := m.Apply(context.Background(), cfg); err == nil {
		t.Fatal("expected error for missing wecom required config")
	}
}

func TestChannelManager_WeComEnabled(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	b := bus.New(10, log)
	reg := wecomTestRegistry(b)
	m := New(b, log, reg)
	cfg := &config.Config{
		Agent: config.AgentConfig{Workspace: t.TempDir()},
		Channels: config.ChannelsConfig{
			WeCom: config.WeComConfig{
				Enabled:        true,
				Token:          "verify-token",
				EncodingAESKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
				AllowFrom:      []string{"zhangsan"},
			},
		},
		Gateway: config.GatewayConfig{Host: config.DefaultHost, Port: config.DefaultPort},
	}
	if err := m.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	found := false
	for _, name := range m.EnabledChannels() {
		if name == "wecom" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("enabled channels does not include wecom: %v", m.EnabledChannels())
	}
}

func TestNewWhatsApp_Disabled(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	b := bus.New(10, log)
	dir := t.TempDir()
	reg := feishuTestRegistry(b)
	m := New(b, log, reg)
	cfg := &config.Config{
		Agent: config.AgentConfig{Workspace: dir},
		Channels: config.ChannelsConfig{
			WhatsApp: config.WhatsAppConfig{
				Enabled:   false,
				StorePath: filepath.Join("/dev/null", "whatsapp-store.db"),
			},
		},
		Gateway: config.GatewayConfig{Host: config.DefaultHost, Port: config.DefaultPort},
	}
	if err := m.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	for _, name := range m.EnabledChannels() {
		if name == "whatsapp" {
			t.Fatalf("%s channel should not be created when disabled", name)
		}
	}
}
