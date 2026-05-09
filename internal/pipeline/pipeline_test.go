package pipeline

import (
	"context"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/session"
	turnctx "github.com/ageneralai/maven/internal/context"
	"github.com/ageneralai/maven/internal/events"
	"github.com/ageneralai/maven/internal/events/eventstest"
	mavenlog "github.com/ageneralai/maven/pkg/log"
)

type stubRuntime struct{}

func (stubRuntime) Run(context.Context, api.Request) (*api.Response, error) {
	return &api.Response{}, nil
}

func (stubRuntime) RunStream(context.Context, api.Request) (<-chan api.StreamEvent, error) {
	ch := make(chan api.StreamEvent)
	close(ch)
	return ch, nil
}

func (stubRuntime) Close() {}

var _ agent.Runtime = stubRuntime{}

func TestHandle_emitsPipelineTurnStartViaRegistry(t *testing.T) {
	t.Cleanup(func() { events.SetDefaultPublisher(nil) })
	b := bus.NewMessageBus(10, mavenlog.Std())
	cap := &eventstest.CapturePublisher{}
	events.SetDefaultPublisher(cap)
	p := New(mavenlog.Std(), b, stubRuntime{}, &session.SessionResolver{}, &agent.PostActionHandler{})
	p.handle(context.Background(), bus.InboundMessage{
		Channel: "telegram",
		ChatID:  "42",
		Content: "hi",
	})
	evs := cap.Snapshot()
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d: %+v", len(evs), evs)
	}
	e := evs[0]
	if e.Type != "pipeline.turn_start" {
		t.Fatalf("Type: %q", e.Type)
	}
	if e.Attrs["channel"] != "telegram" || e.Attrs["chat_id"] != "42" {
		t.Fatalf("Attrs: %+v", e.Attrs)
	}
}

func TestTurnContext_fromInboundMessage_metadataMessageID(t *testing.T) {
	ctx := context.Background()
	msg := bus.InboundMessage{Channel: "telegram", ChatID: "9", Hints: bus.RoutingHints{MessageID: 42}}
	msgCtx := turnctx.WithInbound(ctx, msg.Channel, msg.ChatID)
	if msg.Hints.MessageID != 0 {
		msgCtx = turnctx.WithMetadata(msgCtx, map[string]any{
			"message_id": msg.Hints.MessageID,
		})
	}
	tc, ok := turnctx.From(msgCtx)
	if !ok {
		t.Fatal("From")
	}
	if tc.Channel != "telegram" || tc.ChatID != "9" {
		t.Fatalf("route: %+v", tc)
	}
	id, has := tc.Metadata["message_id"]
	if !has || id != 42 {
		t.Fatalf("message_id: %v has=%v meta=%+v", id, has, tc.Metadata)
	}
}

func TestTurnContext_fromInboundMessage_noMetadataWhenMessageIDZero(t *testing.T) {
	ctx := context.Background()
	msg := bus.InboundMessage{Channel: "telegram", ChatID: "9"}
	msgCtx := turnctx.WithInbound(ctx, msg.Channel, msg.ChatID)
	if msg.Hints.MessageID != 0 {
		msgCtx = turnctx.WithMetadata(msgCtx, map[string]any{
			"message_id": msg.Hints.MessageID,
		})
	}
	tc, ok := turnctx.From(msgCtx)
	if !ok || tc.Metadata != nil {
		t.Fatalf("ok=%v meta=%+v", ok, tc.Metadata)
	}
}
