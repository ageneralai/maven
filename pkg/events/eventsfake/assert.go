package eventsfake

import (
	"testing"

	"github.com/ageneralai/maven/pkg/events"
)

// WantEvent describes expected Type and Attrs for one published event.
type WantEvent struct {
	Type  string
	Attrs map[string]string
}

// AssertPublished compares recorded events to want (order-sensitive).
func AssertPublished(t *testing.T, cap *CapturePublisher, want []WantEvent) {
	t.Helper()
	got := cap.Snapshot()
	if len(got) != len(want) {
		t.Fatalf("event count: got %d want %d: %#v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Type != w.Type {
			t.Fatalf("[%d] Type: got %q want %q", i, got[i].Type, w.Type)
		}
		for k, v := range w.Attrs {
			if got[i].Attrs[k] != v {
				t.Fatalf("[%d] Attrs[%q]: got %q want %q (full %+v)", i, k, got[i].Attrs[k], v, got[i].Attrs)
			}
		}
	}
}

// AssertContainsPublished requires at least one event matching each WantEvent (order-independent).
func AssertContainsPublished(t *testing.T, cap *CapturePublisher, want []WantEvent) {
	t.Helper()
	got := cap.Snapshot()
	for _, w := range want {
		if !containsEvent(got, w) {
			t.Fatalf("missing event Type=%q Attrs=%+v in %#v", w.Type, w.Attrs, got)
		}
	}
}

func containsEvent(got []events.Event, w WantEvent) bool {
	for _, e := range got {
		if e.Type != w.Type {
			continue
		}
		match := true
		for k, v := range w.Attrs {
			if e.Attrs[k] != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
