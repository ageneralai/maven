package log

import (
	"context"
	"log/slog"
	"testing"
)

// Tests must not run in parallel: they share the package-level stdLevel var.

func TestNewRespectsLevel(t *testing.T) {
	logger := New(slog.LevelWarn)
	if logger.Enabled(context.TODO(), slog.LevelInfo) {
		t.Fatal("info should be disabled at warn level")
	}
	if !logger.Enabled(context.TODO(), slog.LevelError) {
		t.Fatal("error should be enabled at warn level")
	}
}

func TestSetLevelUpdatesThreshold(t *testing.T) {
	logger := New(slog.LevelInfo)
	if logger.Enabled(context.TODO(), slog.LevelDebug) {
		t.Fatal("debug should be disabled at info level")
	}
	SetLevel(slog.LevelDebug)
	if !logger.Enabled(context.TODO(), slog.LevelDebug) {
		t.Fatal("debug should be enabled after SetLevel")
	}
}
