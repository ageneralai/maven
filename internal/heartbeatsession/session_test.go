package heartbeatsession

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSessionKey_FormAndUniqueness(t *testing.T) {
	a := SessionKey()
	b := SessionKey()
	if a == b {
		t.Fatal("expected distinct keys per call")
	}
	if !strings.HasPrefix(a, keyPrefix) {
		t.Fatalf("prefix mismatch: %q", a)
	}
	suffix := strings.TrimPrefix(a, keyPrefix)
	if _, err := uuid.Parse(suffix); err != nil {
		t.Fatalf("suffix not uuid: %v", err)
	}
	if !Matches(a) {
		t.Fatal("Matches should accept SessionKey output")
	}
	if Matches("system") || Matches("maven:cron-job-"+suffix) {
		t.Fatal("Matches should reject non-heartbeat keys")
	}
}
