package session

import (
	"strings"
	"testing"
)

func TestChatSessionID(t *testing.T) {
	if got := ChatSessionID("telegram", "12345"); got != "telegram-12345" {
		t.Fatalf("telegram: got %q", got)
	}
	if got := ChatSessionID("web", "550e8400-e29b-41d4-a716-446655440000"); got != "web-550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("web uuid: got %q", got)
	}
	if got := ChatSessionID("web", "web-1"); got != "web-1" {
		t.Fatalf("web idempotent: got %q", got)
	}
}

func TestSessionIDFromRouteKey(t *testing.T) {
	if got := SessionIDFromRouteKey("telegram:12345"); got != "telegram-12345" {
		t.Fatalf("route key: got %q", got)
	}
	if got := SessionIDFromRouteKey("web:web-1"); got != "web-1" {
		t.Fatalf("web route: got %q", got)
	}
}

func TestRotatedSessionID(t *testing.T) {
	got := RotatedSessionID("telegram-12345")
	if got == "telegram-12345" || !strings.HasPrefix(got, "telegram-12345-r") {
		t.Fatalf("rotated: got %q", got)
	}
}
