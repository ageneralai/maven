package cron

import (
	"context"
	"strings"
	"testing"

	turnctx "github.com/ageneralai/maven/internal/kernel/turnctx"
)

func validateToolDeliveryPolicy(ctx context.Context, m map[string]any) error {
	in, err := ParseCronToolInput(m)
	if err != nil {
		return err
	}
	in.ApplyGatewayDeliveryDefaults(ctx)
	return in.ValidateDeliveryPolicy(ctx)
}

func TestCronToolInput_ValidateDeliveryPolicy(t *testing.T) {
	base := map[string]any{
		"name": "x", "message": "y", "in": "1s",
	}
	t.Run("incoming_ok", func(t *testing.T) {
		ctx := turnctx.WithInbound(context.Background(), "telegram", "42")
		m := cloneToolMap(base)
		m["deliver_to_incoming_chat"] = true
		if err := validateToolDeliveryPolicy(ctx, m); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("incoming_missing_ctx", func(t *testing.T) {
		m := cloneToolMap(base)
		m["deliver_to_incoming_chat"] = true
		err := validateToolDeliveryPolicy(context.Background(), m)
		if err == nil || !strings.Contains(err.Error(), "deliver_to_incoming_chat needs") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("incoming_with_channel_rejected", func(t *testing.T) {
		ctx := turnctx.WithInbound(context.Background(), "telegram", "42")
		m := cloneToolMap(base)
		m["deliver_to_incoming_chat"] = true
		m["channel"] = "telegram"
		err := validateToolDeliveryPolicy(ctx, m)
		if err == nil || !strings.Contains(err.Error(), "omit channel") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("incoming_with_to_rejected", func(t *testing.T) {
		ctx := turnctx.WithInbound(context.Background(), "telegram", "42")
		m := cloneToolMap(base)
		m["deliver_to_incoming_chat"] = true
		m["to"] = "999"
		err := validateToolDeliveryPolicy(ctx, m)
		if err == nil || !strings.Contains(err.Error(), "omit channel") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("both_deliver_flags_rejected", func(t *testing.T) {
		ctx := turnctx.WithInbound(context.Background(), "telegram", "42")
		m := cloneToolMap(base)
		m["deliver_to_incoming_chat"] = true
		m["deliver"] = true
		err := validateToolDeliveryPolicy(ctx, m)
		if err == nil || !strings.Contains(err.Error(), "not both") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("explicit_ok", func(t *testing.T) {
		m := cloneToolMap(base)
		m["deliver"] = true
		m["channel"] = "telegram"
		m["to"] = "424242"
		if err := validateToolDeliveryPolicy(context.Background(), m); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("explicit_missing_to", func(t *testing.T) {
		m := cloneToolMap(base)
		m["deliver"] = true
		m["channel"] = "telegram"
		err := validateToolDeliveryPolicy(context.Background(), m)
		if err == nil || !strings.Contains(err.Error(), "requires non-empty channel and to") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("explicit_reserved_to_rejected", func(t *testing.T) {
		m := cloneToolMap(base)
		m["deliver"] = true
		m["channel"] = "telegram"
		m["to"] = "deliver_to_incoming_chat"
		err := validateToolDeliveryPolicy(context.Background(), m)
		if err == nil || !strings.Contains(err.Error(), "invalid to") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("none_ok", func(t *testing.T) {
		m := cloneToolMap(base)
		if err := validateToolDeliveryPolicy(context.Background(), m); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("none_stray_to_rejected", func(t *testing.T) {
		m := cloneToolMap(base)
		m["to"] = "deliver_to_incoming_chat"
		err := validateToolDeliveryPolicy(context.Background(), m)
		if err == nil || !strings.Contains(err.Error(), "omit channel and to") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("none_stray_channel_rejected", func(t *testing.T) {
		m := cloneToolMap(base)
		m["channel"] = "telegram"
		err := validateToolDeliveryPolicy(context.Background(), m)
		if err == nil || !strings.Contains(err.Error(), "omit channel and to") {
			t.Fatalf("got %v", err)
		}
	})
}

func cloneToolMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func TestCronToolInput_ToAddParams_messageIDFromTurnMetadata(t *testing.T) {
	ctx := turnctx.WithInbound(context.Background(), "telegram", "42")
	ctx = turnctx.WithMetadata(ctx, map[string]any{"message_id": 99})
	in := CronToolInput{
		Name:                  "n",
		Message:               "m",
		In:                    "1s",
		Deliver:               true,
		DeliverToIncomingChat: true,
	}
	p := in.ToAddParams(ctx)
	if p.Channel != "telegram" || p.To != "42" {
		t.Fatalf("route: %+v", p)
	}
	if p.MessageID != 99 {
		t.Fatalf("MessageID=%d", p.MessageID)
	}
}

func TestCronToolInput_ToAddParams_messageIDZeroWhenMetadataAbsent(t *testing.T) {
	ctx := turnctx.WithInbound(context.Background(), "telegram", "42")
	in := CronToolInput{
		Deliver:               true,
		DeliverToIncomingChat: true,
	}
	p := in.ToAddParams(ctx)
	if p.MessageID != 0 {
		t.Fatalf("MessageID=%d", p.MessageID)
	}
}
