package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw   string
		level slog.Level
		err   bool
	}{
		{"", slog.LevelInfo, false},
		{"info", slog.LevelInfo, false},
		{"DEBUG", slog.LevelDebug, false},
		{"warn", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"trace", slog.LevelInfo, true},
	}
	for _, tc := range tests {
		level, err := ParseLogLevel(tc.raw)
		if tc.err {
			if err == nil {
				t.Fatalf("ParseLogLevel(%q) expected error", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseLogLevel(%q): %v", tc.raw, err)
		}
		if level != tc.level {
			t.Fatalf("ParseLogLevel(%q) = %v, want %v", tc.raw, level, tc.level)
		}
	}
}

func TestLoggingConfig_Validate(t *testing.T) {
	t.Parallel()
	if err := (LoggingConfig{Level: "debug"}).Validate(); err != nil {
		t.Fatalf("Validate(debug): %v", err)
	}
	if err := (LoggingConfig{Level: "nope"}).Validate(); err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestApplyEnv_MAVEN_LOG_LEVEL(t *testing.T) {
	cfg := DefaultConfig()
	t.Setenv("MAVEN_LOG_LEVEL", "debug")
	applyEnv(cfg)
	if cfg.Logging.Level != "debug" {
		t.Fatalf("logging.level = %q, want debug", cfg.Logging.Level)
	}
}

func TestBootstrapLogLevel_FromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("MAVEN_LOG_LEVEL", "")
	cfgDir := filepath.Join(tmpDir, ".maven")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"logging":{"level":"warn"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	level, err := BootstrapLogLevel()
	if err != nil {
		t.Fatalf("BootstrapLogLevel: %v", err)
	}
	if level != slog.LevelWarn {
		t.Fatalf("level = %v, want warn", level)
	}
}

func TestBootstrapLogLevel_EnvOverridesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("MAVEN_LOG_LEVEL", "error")
	cfgDir := filepath.Join(tmpDir, ".maven")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"logging":{"level":"debug"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	level, err := BootstrapLogLevel()
	if err != nil {
		t.Fatalf("BootstrapLogLevel: %v", err)
	}
	if level != slog.LevelError {
		t.Fatalf("level = %v, want error", level)
	}
}
