package cronsession

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSessionKey_FormAndUniqueness(t *testing.T) {
	jobID := "job-abc"
	a := SessionKey(jobID)
	b := SessionKey(jobID)
	if a == b {
		t.Fatal("expected distinct keys per call")
	}
	if !strings.HasPrefix(a, keyPrefix+jobID+"-") {
		t.Fatalf("prefix mismatch: %q", a)
	}
	suffix := strings.TrimPrefix(a, keyPrefix+jobID+"-")
	if _, err := uuid.Parse(suffix); err != nil {
		t.Fatalf("suffix not uuid: %v", err)
	}
	if !MatchesJob(jobID, a) {
		t.Fatal("MatchesJob should accept SessionKey output")
	}
	if MatchesJob("other", a) {
		t.Fatal("MatchesJob should reject wrong job id")
	}
}
