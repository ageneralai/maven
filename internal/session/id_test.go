package session

import (
	"testing"
)

func TestChatSessionID(t *testing.T) {
	t.Parallel()
	if got := ChatSessionID("telegram", "12345"); got != "telegram-12345" {
		t.Fatalf("got %q", got)
	}
	if got := ChatSessionID("web", "550e8400-e29b-41d4-a716-446655440000"); got != "web-550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("got %q", got)
	}
	if got := ChatSessionID("web", "web-1"); got != "web-1" {
		t.Fatalf("got %q", got)
	}
}

func TestSessionIDFromRouteKey(t *testing.T) {
	t.Parallel()
	if got := SessionIDFromRouteKey("telegram:12345"); got != "telegram-12345" {
		t.Fatalf("got %q", got)
	}
	if got := SessionIDFromRouteKey("web:web-1"); got != "web-1" {
		t.Fatalf("got %q", got)
	}
}
