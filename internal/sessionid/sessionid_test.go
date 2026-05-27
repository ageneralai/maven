package sessionid

import (
	"strings"
	"testing"
)

func TestNewCron_FormAndUniqueness(t *testing.T) {
	t.Parallel()
	jobID := "job-abc"
	a := New(KindCron, jobID)
	b := New(KindCron, jobID)
	if a.String() == b.String() {
		t.Fatal("expected unique cron session ids")
	}
	if !strings.HasPrefix(a.String(), "cron:"+jobID+":") {
		t.Fatalf("unexpected form: %q", a.String())
	}
	parsed, err := Parse(a.String())
	if err != nil || parsed.Kind != KindCron || parsed.Owner != jobID {
		t.Fatalf("Parse cron: %+v err=%v", parsed, err)
	}
	if other, err := Parse(a.String()); err != nil || other.Owner != jobID {
		t.Fatal("Parse should preserve job id")
	}
}

func TestNewHeartbeat_FormAndUniqueness(t *testing.T) {
	t.Parallel()
	a := New(KindHeartbeat, "")
	b := New(KindHeartbeat, "")
	if a.String() == b.String() {
		t.Fatal("expected unique heartbeat session ids")
	}
	if !strings.HasPrefix(a.String(), "heartbeat:") {
		t.Fatalf("unexpected form: %q", a.String())
	}
	parsed, err := Parse(a.String())
	if err != nil || parsed.Kind != KindHeartbeat {
		t.Fatalf("Parse heartbeat: %+v err=%v", parsed, err)
	}
	if id, err := Parse("system"); err == nil && id.Kind == KindHeartbeat {
		t.Fatal("Parse should reject non-heartbeat keys")
	}
}

func TestNewRotatedAndIsolated(t *testing.T) {
	t.Parallel()
	base := "telegram-12345"
	rotated := New(KindRotated, base)
	if !strings.HasPrefix(rotated.String(), base+":rotated:") {
		t.Fatalf("rotated form: %q", rotated.String())
	}
	parsed, err := Parse(rotated.String())
	if err != nil || parsed.Kind != KindRotated || parsed.Owner != base {
		t.Fatalf("Parse rotated = %+v err=%v", parsed, err)
	}
	isolated := New(KindIsolated, base)
	if !strings.HasPrefix(isolated.String(), base+":isolated:") {
		t.Fatalf("isolated form: %q", isolated.String())
	}
}

func TestNewTask(t *testing.T) {
	t.Parallel()
	id := New(KindTask, "")
	if !MatchesTask(id.String()) {
		t.Fatalf("expected task session id, got %q", id.String())
	}
}

func TestParseRoundTrip(t *testing.T) {
	t.Parallel()
	cases := []SessionID{
		New(KindCron, "job-1"),
		New(KindHeartbeat, ""),
		New(KindTask, ""),
		New(KindIsolated, "telegram-1"),
		New(KindRotated, "telegram-1"),
		New(KindChat, "telegram-99"),
	}
	for _, id := range cases {
		got, err := Parse(id.String())
		if err != nil {
			t.Fatalf("Parse(%q): %v", id.String(), err)
		}
		if got.Kind != id.Kind || got.Owner != id.Owner {
			t.Fatalf("Parse(%q) = %+v want %+v", id.String(), got, id)
		}
	}
}
