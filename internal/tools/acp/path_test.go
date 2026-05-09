package acp

import (
	"path/filepath"
	"testing"
)

func TestResolveWorkspacePath_unrestricted(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "..", "outside-restrict")
	got, err := resolveWorkspacePath(dir, false, outside)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("want absolute path, got %q", got)
	}
}

func TestResolveWorkspacePath_restrictDeniesEscape(t *testing.T) {
	ws := t.TempDir()
	parent := filepath.Dir(ws)
	_, err := resolveWorkspacePath(ws, true, parent)
	if err == nil {
		t.Fatal("expected error for path outside workspace")
	}
}

func TestResolveWorkspacePath_restrictAllowsInside(t *testing.T) {
	ws := t.TempDir()
	sub := filepath.Join(ws, "pkg")
	got, err := resolveWorkspacePath(ws, true, sub)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("want absolute path, got %q", got)
	}
}
