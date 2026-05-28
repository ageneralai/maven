package gateway

import (
	"log/slog"
	"testing"

	"github.com/ageneralai/maven/internal/kernel/config"
	mavenlog "github.com/ageneralai/maven/internal/kernel/log"
)

func TestApplyLogLevel(t *testing.T) {
	t.Parallel()
	logger := mavenlog.New(slog.LevelInfo)
	g := &Gateway{logger: logger}
	cfg := config.DefaultConfig()
	cfg.Logging.Level = "debug"
	if err := g.applyLogLevel(cfg); err != nil {
		t.Fatalf("applyLogLevel(debug): %v", err)
	}
	if !logger.Enabled(nil, slog.LevelDebug) {
		t.Fatal("expected debug enabled after applyLogLevel")
	}
	cfg.Logging.Level = "bogus"
	if err := g.applyLogLevel(cfg); err == nil {
		t.Fatal("expected error for invalid level")
	}
}
