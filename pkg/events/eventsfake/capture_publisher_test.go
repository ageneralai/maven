package eventsfake

import (
	"context"
	"testing"

	"github.com/ageneralai/maven/pkg/events"
)

func TestCapturePublisher_RecordsEvents(t *testing.T) {
	t.Parallel()
	c := &CapturePublisher{}
	c.Publish(context.Background(), events.Event{Type: events.EventBusPublishFailure})
	AssertPublished(t, c, []WantEvent{{Type: events.EventBusPublishFailure}})
}
