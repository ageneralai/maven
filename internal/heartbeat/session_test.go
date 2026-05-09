package heartbeat

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestHeartbeatSessionKey_FormAndUniqueness(t *testing.T) {
	a := SessionKey()
	b := SessionKey()
	if a == b {
		t.Fatal("expected distinct keys per call")
	}
	if !strings.HasPrefix(a, heartbeatSessionKeyPrefix) {
		t.Fatalf("prefix mismatch: %q", a)
	}
	suffix := strings.TrimPrefix(a, heartbeatSessionKeyPrefix)
	if _, err := uuid.Parse(suffix); err != nil {
		t.Fatalf("suffix not uuid: %v", err)
	}
	if !MatchesSession(a) {
		t.Fatal("MatchesSession should accept SessionKey output")
	}
	if MatchesSession("system") || MatchesSession("maven:cron-job-"+suffix) {
		t.Fatal("MatchesSession should reject non-heartbeat keys")
	}
}
