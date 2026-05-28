package session

import (
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/message"
)

func TestNoIsolatedStore_SkipsIsolated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inner, _ := NewStore(dir)
	s := NewNoIsolatedStore(inner)
	msgs := []message.Message{{Role: "user", Content: "hi"}}
	isolatedID := "mem-consolidate:isolated:1748000000000000000"
	if err := s.Save(isolatedID, msgs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load(isolatedID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no messages for isolated session, got %d", len(got))
	}
}

func TestNoIsolatedStore_PersistsChat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inner, _ := NewStore(dir)
	s := NewNoIsolatedStore(inner)
	msgs := []message.Message{{Role: "user", Content: "hi"}}
	if err := s.Save("telegram-42", msgs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load("telegram-42")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
}

func TestNoIsolatedStore_PersistsCron(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inner, _ := NewStore(dir)
	s := NewNoIsolatedStore(inner)
	msgs := []message.Message{{Role: "user", Content: "hi"}}
	cronID := "cron:daily-report:550e8400-e29b-41d4-a716-446655440000"
	if err := s.Save(cronID, msgs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load(cronID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 message for cron session, got %d", len(got))
	}
}
