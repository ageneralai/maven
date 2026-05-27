package allowlist

import "testing"

func TestMatcher_Allow_NoFilter(t *testing.T) {
	t.Parallel()
	m := NewMatcher(nil)
	if !m.Allow("anyone") {
		t.Error("should allow anyone when matcher is empty")
	}
}

func TestMatcher_Allow_WithFilter(t *testing.T) {
	t.Parallel()
	m := NewMatcher([]string{"user1", "user2"})
	if !m.Allow("user1") {
		t.Error("should allow user1")
	}
	if !m.Allow("user2") {
		t.Error("should allow user2")
	}
	if m.Allow("user3") {
		t.Error("should reject user3")
	}
}

func TestNewMatcher_TrimsAndDedups(t *testing.T) {
	t.Parallel()
	m := NewMatcher([]string{" user1 ", "user1", "", "user2", " user2 "})
	if len(m) != 2 {
		t.Fatalf("len(m) = %d, want 2", len(m))
	}
	if !m.Allow("user1") || !m.Allow("user2") {
		t.Error("expected trimmed ids in matcher")
	}
}
