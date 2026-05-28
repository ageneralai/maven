package gateway

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/agent"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/slash"
	"log/slog"
)

func capturingRuntimeFactory(last *atomic.Value) RuntimeFactory {
	return func(_ *config.Config, sysPrompt string, _ []api.SkillRegistration, _ []tool.Tool, _ api.SessionStore, _ *slog.Logger) (agent.Runtime, error) {
		last.Store(sysPrompt)
		return &mockRuntime{}, nil
	}
}

func TestGateway_ReloadFromConfig_ReReadsAgentsMD(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ws := filepath.Join(home, "workspace")
	if err := os.MkdirAll(ws, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(home, ".maven")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgOnDisk := map[string]any{"agent": map[string]any{"workspace": ws}}
	data, err := json.Marshal(cfgOnDisk)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	agentsPath := filepath.Join(ws, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# Agent v1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var lastPrompt atomic.Value
	cfg := &config.Config{
		Agent:    config.AgentConfig{Workspace: ws},
		Channels: config.ChannelsConfig{},
	}
	g, err := NewWithOptions(cfg, Options{Logger: testLG, RuntimeFactory: capturingRuntimeFactory(&lastPrompt)})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	defer func() { _ = g.Shutdown() }()
	if err := g.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	p1, ok := lastPrompt.Load().(string)
	if !ok || !strings.Contains(p1, "v1") {
		t.Fatalf("first prompt %q", p1)
	}
	if err := os.WriteFile(agentsPath, []byte("# Agent v2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	g.reloadFromConfig(context.Background())
	p2, ok := lastPrompt.Load().(string)
	if !ok || !strings.Contains(p2, "v2") {
		t.Fatalf("reloaded prompt %q", p2)
	}
	if strings.Contains(p2, "v1") {
		t.Fatalf("stale prompt still contains v1: %q", p2)
	}
}

func TestGateway_RequestReload_Coalesces(t *testing.T) {
	g := &Gateway{manualReloadCh: make(chan struct{}, 1)}
	g.requestReload()
	g.requestReload()
	select {
	case <-g.manualReloadCh:
	default:
		t.Fatal("expected one pending reload signal")
	}
	select {
	case <-g.manualReloadCh:
		t.Fatal("expected coalesced duplicate reload")
	default:
	}
}

func TestGateway_ReloadSlash_SignalsChannel(t *testing.T) {
	g := &Gateway{manualReloadCh: make(chan struct{}, 1), logger: testLG}
	reg, err := slash.BuiltIns()
	if err != nil {
		t.Fatal(err)
	}
	if err := g.registerReloadSlash(reg); err != nil {
		t.Fatal(err)
	}
	out, err := slash.PreTurn(context.Background(), reg, slash.Input{Text: "/reload"})
	if err != nil {
		t.Fatal(err)
	}
	if out.DirectReply != "Reloading…" {
		t.Fatalf("DirectReply got %q", out.DirectReply)
	}
	select {
	case <-g.manualReloadCh:
	default:
		t.Fatal("reload slash did not signal manualReloadCh")
	}
}
