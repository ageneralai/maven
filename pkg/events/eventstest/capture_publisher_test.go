package eventstest

import (
	"context"
	"testing"

	"github.com/ageneralai/maven/pkg/events"
)

func TestCapturePublisher_RecordsEvents(t *testing.T) {
	c := &CapturePublisher{}
	c.Publish(context.Background(), events.Event{Type: events.EventBusPublishFailure})
	got := c.Snapshot()
	if len(got) != 1 || got[0].Type != events.EventBusPublishFailure {
		t.Fatalf("got %#v", got)
	}
}
