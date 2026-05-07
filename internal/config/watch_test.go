package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchConfigPath_StopIdempotent(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config.json")
	_ = os.WriteFile(p, []byte("{}"), 0644)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, stop := WatchConfigPath(ctx, p, 50*time.Millisecond)
	stop()
	stop()
	select {
	case <-ch:
	case <-time.After(30 * time.Millisecond):
	}
}

func TestWatchConfigPath_EmitsOnWrite(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(p, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	debounce := 80 * time.Millisecond
	ch, stop := WatchConfigPath(ctx, p, debounce)
	defer stop()
	if err := os.WriteFile(p, []byte(`{"x":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-ctx.Done():
		t.Fatal("timeout waiting for debounced reload signal")
	}
}

func TestFileModEpoch(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "f")
	if FileModEpoch(p) != 0 {
		t.Fatal("missing file should be 0")
	}
	_ = os.WriteFile(p, []byte("a"), 0644)
	if FileModEpoch(p) == 0 {
		t.Fatal("expected non-zero epoch")
	}
}
