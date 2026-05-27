package sessionid

import (
	"strings"
	"testing"
)

func TestNewCron_FormAndUniqueness(t *testing.T) {
	jobID := "job-abc"
	a := New(KindCron, jobID)
	b := New(KindCron, jobID)
	if a == b {
		t.Fatal("expected unique cron session ids")
	}
	if !strings.HasPrefix(a, "cron:"+jobID+":") {
		t.Fatalf("unexpected form: %q", a)
	}
	if !MatchesCronJob(jobID, a) {
		t.Fatal("MatchesCronJob should accept New output")
	}
	if MatchesCronJob("other", a) {
		t.Fatal("MatchesCronJob should reject wrong job id")
	}
}

func TestNewHeartbeat_FormAndUniqueness(t *testing.T) {
	a := New(KindHeartbeat, "")
	b := New(KindHeartbeat, "")
	if a == b {
		t.Fatal("expected unique heartbeat session ids")
	}
	if !strings.HasPrefix(a, "heartbeat:") {
		t.Fatalf("unexpected form: %q", a)
	}
	if !MatchesHeartbeat(a) {
		t.Fatal("MatchesHeartbeat should accept New output")
	}
	if MatchesHeartbeat("system") {
		t.Fatal("MatchesHeartbeat should reject non-heartbeat keys")
	}
}

func TestNewRotatedAndIsolated(t *testing.T) {
	base := "telegram-12345"
	rotated := New(KindRotated, base)
	if !strings.HasPrefix(rotated, base+":rotated:") {
		t.Fatalf("rotated form: %q", rotated)
	}
	kind, seed, ok := Match(rotated)
	if !ok || kind != KindRotated || seed != base {
		t.Fatalf("Match rotated = %q %q %v", kind, seed, ok)
	}
	isolated := New(KindIsolated, base)
	if !strings.HasPrefix(isolated, base+":isolated:") {
		t.Fatalf("isolated form: %q", isolated)
	}
}

func TestNewTask(t *testing.T) {
	id := New(KindTask, "")
	if !MatchesTask(id) {
		t.Fatalf("expected task session id, got %q", id)
	}
}
