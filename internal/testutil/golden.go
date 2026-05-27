package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// AssertGoldenFile compares got to testdata/name; set UPDATE_GOLDEN=1 to rewrite.
func AssertGoldenFile(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden %s mismatch\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}
