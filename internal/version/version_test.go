package version

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	Version = "v0.1.10"
	Commit = "abc1234"
	Date = "2026-05-25T00:00:00Z"
	t.Cleanup(func() {
		Version = "dev"
		Commit = "none"
		Date = "unknown"
	})
	info := Load()
	if info.Version != "v0.1.10" {
		t.Fatalf("Version=%q", info.Version)
	}
	if info.Commit != "abc1234" {
		t.Fatalf("Commit=%q", info.Commit)
	}
	if info.Date != "2026-05-25T00:00:00Z" {
		t.Fatalf("Date=%q", info.Date)
	}
	if info.Go == "" || info.Go == "unknown" {
		t.Fatalf("Go=%q", info.Go)
	}
	lines := info.Lines()
	if !strings.Contains(lines[1], "v0.1.10") {
		t.Fatalf("maven line: %s", lines[1])
	}
	if !strings.Contains(lines[2], SDKModule) {
		t.Fatalf("sdk line: %s", lines[2])
	}
}

func TestModuleVersionReplaceLocal(t *testing.T) {
	got := moduleVersion(&debug.Module{Replace: &debug.Module{Version: ""}})
	if got != "(local)" {
		t.Fatalf("got %q", got)
	}
}
