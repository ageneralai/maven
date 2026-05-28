package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if cfg.Agent.Model != DefaultModel {
		t.Errorf("model = %q, want %q", cfg.Agent.Model, DefaultModel)
	}
	if cfg.Agent.MaxTokens != DefaultMaxTokens {
		t.Errorf("maxTokens = %d, want %d", cfg.Agent.MaxTokens, DefaultMaxTokens)
	}
	if cfg.Agent.MaxToolIterations != DefaultMaxToolIterations {
		t.Errorf("maxToolIterations = %d, want %d", cfg.Agent.MaxToolIterations, DefaultMaxToolIterations)
	}
	if cfg.Gateway.Host != DefaultHost {
		t.Errorf("host = %q, want %q", cfg.Gateway.Host, DefaultHost)
	}
	if cfg.Gateway.Port != DefaultPort {
		t.Errorf("port = %d, want %d", cfg.Gateway.Port, DefaultPort)
	}
	if cfg.Tools.ExecTimeout != DefaultExecTimeout {
		t.Errorf("execTimeout = %d, want %d", cfg.Tools.ExecTimeout, DefaultExecTimeout)
	}
	if !cfg.Tools.RestrictToWorkspace {
		t.Error("restrictToWorkspace should be true by default")
	}
	if !cfg.Skills.Enabled {
		t.Error("skills.enabled should be true by default")
	}
	if cfg.AutoCompact.Enabled {
		t.Error("autoCompact.enabled should be false by default (opt-in)")
	}
	if cfg.AutoCompact.Threshold != 0.8 {
		t.Errorf("autoCompact.threshold = %v, want 0.8", cfg.AutoCompact.Threshold)
	}
	if cfg.AutoCompact.PreserveCount != 5 {
		t.Errorf("autoCompact.preserveCount = %d, want 5", cfg.AutoCompact.PreserveCount)
	}
	if cfg.Agent.Workspace == "" {
		t.Error("workspace should not be empty")
	}
	if cfg.ShadowJournal.Enabled {
		t.Error("shadowJournal.enabled should be false by default")
	}
	if cfg.ShadowJournal.Model != "" {
		t.Errorf("shadowJournal.model = %q, want empty", cfg.ShadowJournal.Model)
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Agent.Model != DefaultModel {
		t.Errorf("expected default model %q, got %q", DefaultModel, cfg.Agent.Model)
	}
}

func TestLoadConfig_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create config file
	cfgDir := filepath.Join(tmpDir, ".maven")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	testCfg := map[string]any{
		"agent": map[string]any{
			"model":     "claude-opus-4-20250514",
			"maxTokens": 4096,
		},
		"provider": map[string]any{
			"apiKey": "sk-test-key",
		},
	}
	data, _ := json.MarshalIndent(testCfg, "", "  ")
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Agent.Model != "claude-opus-4-20250514" {
		t.Errorf("model = %q, want claude-opus-4-20250514", cfg.Agent.Model)
	}
	if cfg.Agent.MaxTokens != 4096 {
		t.Errorf("maxTokens = %d, want 4096", cfg.Agent.MaxTokens)
	}
	if cfg.Provider.APIKey != "sk-test-key" {
		t.Errorf("apiKey = %q, want sk-test-key", cfg.Provider.APIKey)
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := DefaultConfig()
	cfg.Provider.APIKey = "test-key"

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".maven", "config.json"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal saved config: %v", err)
	}
	if loaded.Provider.APIKey != "test-key" {
		t.Errorf("saved apiKey = %q, want test-key", loaded.Provider.APIKey)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfgDir := filepath.Join(tmpDir, ".maven")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadConfig_EmptyWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfgDir := filepath.Join(tmpDir, ".maven")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Config with empty workspace - should use default
	testCfg := map[string]any{
		"agent": map[string]any{
			"workspace": "",
		},
	}
	data, _ := json.MarshalIndent(testCfg, "", "  ")
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Agent.Workspace == "" {
		t.Error("workspace should not be empty")
	}
}

func validTestConfig() *Config {
	cfg := DefaultConfig()
	cfg.Provider.APIKey = "test-api-key"
	return cfg
}

func TestConfig_Validate_GatewayPort(t *testing.T) {
	t.Parallel()
	cfg := validTestConfig()
	cfg.Gateway.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for gateway.port 0")
	}
}

func TestConfig_Validate_GatewayCronMaxConcurrentRuns(t *testing.T) {
	t.Parallel()
	cfg := validTestConfig()
	cfg.Gateway.Cron.MaxConcurrentRuns = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for gateway.cron.maxConcurrentRuns < 0")
	}
	cfg.Gateway.Cron.MaxConcurrentRuns = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("0 should validate: %v", err)
	}
}

func TestConfig_Validate_AutoCompactThreshold(t *testing.T) {
	t.Parallel()
	cfg := validTestConfig()
	cfg.AutoCompact.Enabled = true
	cfg.AutoCompact.Threshold = 1.5
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for autoCompact.threshold > 1")
	}
}

func TestProviderConfig_Validate(t *testing.T) {
	t.Parallel()
	if err := (ProviderConfig{}).Validate(); err == nil {
		t.Fatal("expected missing api key error")
	}
}

func TestAgentConfig_Validate(t *testing.T) {
	t.Parallel()
	cfg := AgentConfig{Workspace: "/tmp", MaxTokens: 100, MaxToolIterations: 1}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid agent config: %v", err)
	}
	cfg.MaxTokens = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected maxTokens error")
	}
}

func TestGatewayConfig_Validate(t *testing.T) {
	t.Parallel()
	cfg := GatewayConfig{Host: "127.0.0.1", Port: 8080}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid gateway config: %v", err)
	}
	cfg.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected port error")
	}
}

func TestWeComConfig_Validate_EncodingAESKeyLength(t *testing.T) {
	t.Parallel()
	cfg := validTestConfig()
	cfg.Channels.WeCom = WeComConfig{
		Enabled:        true,
		Token:          "token",
		EncodingAESKey: "short",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected encodingAESKey length error")
	}
	cfg.Channels.WeCom.EncodingAESKey = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid wecom config: %v", err)
	}
}

func TestTelegramConfig_Validate_TokenFormat(t *testing.T) {
	t.Parallel()
	cfg := validTestConfig()
	cfg.Channels.Telegram = TelegramConfig{Enabled: true, Token: "bad-token"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected telegram token format error")
	}
	cfg.Channels.Telegram.Token = "1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefgh"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid telegram token: %v", err)
	}
}

func TestMatrixConfig_Validate_UserIDAndHomeserver(t *testing.T) {
	t.Parallel()
	cfg := validTestConfig()
	cfg.Channels.Matrix = MatrixConfig{
		Enabled:     true,
		Homeserver:  "not-a-url",
		AccessToken: "tok",
		UserID:      "user",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected matrix validation error")
	}
	cfg.Channels.Matrix.Homeserver = "https://matrix.example.org"
	cfg.Channels.Matrix.UserID = "@user:example.org"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid matrix config: %v", err)
	}
}

func TestChannelsConfig_Validate(t *testing.T) {
	t.Parallel()
	cfg := ChannelsConfig{Telegram: TelegramConfig{Enabled: true}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected telegram token error")
	}
}

func TestAutoCompactConfig_Validate(t *testing.T) {
	t.Parallel()
	cfg := AutoCompactConfig{Enabled: true, Threshold: 0.5, PreserveCount: 1}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid auto compact: %v", err)
	}
}
