package eventsfake

import (
	"context"
	"testing"

	"github.com/ageneralai/maven/pkg/events"
)

func TestAssertPublished_MatchesTypeAndAttrs(t *testing.T) {
	t.Parallel()
	c := &CapturePublisher{}
	c.Publish(context.Background(), events.Event{
		Type:  events.EventBusPublishFailure,
		Attrs: map[string]string{"stream": "outbound", "channel": "a"},
	})
	AssertPublished(t, c, []WantEvent{{
		Type:  events.EventBusPublishFailure,
		Attrs: map[string]string{"stream": "outbound", "channel": "a"},
	}})
}

func TestAssertContainsPublished_OrderIndependent(t *testing.T) {
	t.Parallel()
	c := &CapturePublisher{}
	c.Publish(context.Background(), events.Event{Type: events.EventBusClosed})
	c.Publish(context.Background(), events.Event{
		Type:  events.EventBusPublishFailure,
		Attrs: map[string]string{"stream": "inbound"},
	})
	AssertContainsPublished(t, c, []WantEvent{
		{Type: events.EventBusClosed},
		{Type: events.EventBusPublishFailure, Attrs: map[string]string{"stream": "inbound"}},
	})
}
