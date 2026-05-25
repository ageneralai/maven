package matrix

import (
	"context"
	"path/filepath"
	"testing"

	"maunium.net/go/mautrix/id"
)

func TestFileSyncStore_PersistsNextBatch(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "ws")
	userID := id.UserID("@agent:example.org")
	store, err := openFileSyncStore(workspace, userID, "MAVEN01")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if got := store.DeviceID(); got != "MAVEN01" {
		t.Fatalf("device id = %q, want MAVEN01", got)
	}
	ctx := context.Background()
	if err := store.SaveNextBatch(ctx, userID, "s123"); err != nil {
		t.Fatalf("save next batch: %v", err)
	}
	if err := store.SaveFilterID(ctx, userID, "f456"); err != nil {
		t.Fatalf("save filter: %v", err)
	}
	reloaded, err := openFileSyncStore(workspace, userID, "MAVEN01")
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	gotBatch, err := reloaded.LoadNextBatch(ctx, userID)
	if err != nil || gotBatch != "s123" {
		t.Fatalf("next batch = %q err=%v", gotBatch, err)
	}
	gotFilter, err := reloaded.LoadFilterID(ctx, userID)
	if err != nil || gotFilter != "f456" {
		t.Fatalf("filter id = %q err=%v", gotFilter, err)
	}
}

func TestFileSyncStore_GeneratesDeviceID(t *testing.T) {
	dir := t.TempDir()
	userID := id.UserID("@agent:example.org")
	store, err := openFileSyncStore(dir, userID, "")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if store.DeviceID() == "" {
		t.Fatal("expected generated device id")
	}
	reloaded, err := openFileSyncStore(dir, userID, "")
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	if reloaded.DeviceID() != store.DeviceID() {
		t.Fatalf("device id changed on reload: %q vs %q", reloaded.DeviceID(), store.DeviceID())
	}
}
