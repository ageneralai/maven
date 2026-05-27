package channel

import (
	"testing"

	"github.com/ageneralai/maven/internal/channel/allowlist"
)

func TestAllowlistMatcher_Allow_NoFilter(t *testing.T) {
	t.Parallel()
	m := allowlist.NewMatcher(nil)
	if !m.Allow("anyone") {
		t.Error("should allow anyone when allowFrom is empty")
	}
}

func TestAllowlistMatcher_Allow_WithFilter(t *testing.T) {
	t.Parallel()
	m := allowlist.NewMatcher([]string{"user1", "user2"})
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

func TestAllowlistMatcher_TrimsAndDedups(t *testing.T) {
	t.Parallel()
	m := allowlist.NewMatcher([]string{" user1 ", "user1", "", "user2"})
	if len(m) != 2 {
		t.Fatalf("len(m) = %d, want 2", len(m))
	}
}
