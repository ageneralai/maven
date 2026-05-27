package pipeline

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/agent/postaction"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/session"
	turnctx "github.com/ageneralai/maven/pkg/context"
	"github.com/ageneralai/maven/pkg/events"
	"github.com/ageneralai/maven/pkg/events/eventsfake"
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
	tests := []struct {
		name    string
		msg     bus.InboundMessage
		want    []eventsfake.WantEvent
	}{
		{
			name: "telegram_chat",
			msg: bus.InboundMessage{
				Channel: "telegram",
				ChatID:  "42",
				Content: "hi",
			},
			want: []eventsfake.WantEvent{{
				Type: "pipeline.turn_start",
				Attrs: map[string]string{
					"channel": "telegram",
					"chat_id": "42",
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { events.SetDefaultPublisher(nil) })
			b := bus.New(10, slog.New(slog.DiscardHandler))
			cap := &eventsfake.CapturePublisher{}
			events.SetDefaultPublisher(cap)
			router, _ := session.New("")
			p := New(slog.New(slog.DiscardHandler), b, stubRuntime{}, &session.SessionResolver{Router: router}, postaction.New(router, ""))
			p.handle(context.Background(), tt.msg)
			eventsfake.AssertPublished(t, cap, tt.want)
		})
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
