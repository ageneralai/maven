package session

import (
	"os"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/message"
)

func TestStore_LoadSave(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Load non-existent session returns nil.
	msgs, err := s.Load("no-such-session")
	if err != nil || msgs != nil {
		t.Fatalf("expected nil, nil; got %v, %v", msgs, err)
	}
	// Save and reload.
	want := []message.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	if err := s.Save("sess1", want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("sess1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d messages, want %d", len(got), len(want))
	}
	for i, m := range got {
		if m.Role != want[i].Role || m.Content != want[i].Content {
			t.Fatalf("message %d mismatch: got %+v want %+v", i, m, want[i])
		}
	}
}

func TestStore_SaveEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, _ := NewStore(dir)
	// Save empty slice should not create file.
	_ = s.Save("sess", nil)
	if _, err := os.Stat(s.path("sess")); !os.IsNotExist(err) {
		t.Fatal("expected no file for empty save")
	}
}

func TestStore_PathSanitize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, _ := NewStore(dir)
	// Session IDs with slashes/colons should not escape dir.
	msgs := []message.Message{{Role: "user", Content: "x"}}
	_ = s.Save("a/b:c", msgs)
	got, _ := s.Load("a/b:c")
	if len(got) != 1 {
		t.Fatalf("got %d want 1", len(got))
	}
}
