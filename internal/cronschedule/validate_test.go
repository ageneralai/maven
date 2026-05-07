package cronschedule

import (
	"context"
	"strings"
	"testing"

	"github.com/stellarlinkco/maven/internal/inboundctx"
)

func TestValidateCronDelivery(t *testing.T) {
	base := map[string]interface{}{
		"name": "x", "message": "y", "in": "1s",
	}
	t.Run("incoming_ok", func(t *testing.T) {
		ctx := inboundctx.With(context.Background(), "telegram", "42")
		m := cloneMap(base)
		m["deliver_to_incoming_chat"] = true
		if err := validateCronDelivery(ctx, m); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("incoming_missing_ctx", func(t *testing.T) {
		m := cloneMap(base)
		m["deliver_to_incoming_chat"] = true
		err := validateCronDelivery(context.Background(), m)
		if err == nil || !strings.Contains(err.Error(), "deliver_to_incoming_chat needs") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("incoming_with_channel_rejected", func(t *testing.T) {
		ctx := inboundctx.With(context.Background(), "telegram", "42")
		m := cloneMap(base)
		m["deliver_to_incoming_chat"] = true
		m["channel"] = "telegram"
		err := validateCronDelivery(ctx, m)
		if err == nil || !strings.Contains(err.Error(), "omit channel") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("incoming_with_to_rejected", func(t *testing.T) {
		ctx := inboundctx.With(context.Background(), "telegram", "42")
		m := cloneMap(base)
		m["deliver_to_incoming_chat"] = true
		m["to"] = "999"
		err := validateCronDelivery(ctx, m)
		if err == nil || !strings.Contains(err.Error(), "omit channel") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("both_deliver_flags_rejected", func(t *testing.T) {
		ctx := inboundctx.With(context.Background(), "telegram", "42")
		m := cloneMap(base)
		m["deliver_to_incoming_chat"] = true
		m["deliver"] = true
		err := validateCronDelivery(ctx, m)
		if err == nil || !strings.Contains(err.Error(), "not both") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("explicit_ok", func(t *testing.T) {
		m := cloneMap(base)
		m["deliver"] = true
		m["channel"] = "telegram"
		m["to"] = "424242"
		if err := validateCronDelivery(context.Background(), m); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("explicit_missing_to", func(t *testing.T) {
		m := cloneMap(base)
		m["deliver"] = true
		m["channel"] = "telegram"
		err := validateCronDelivery(context.Background(), m)
		if err == nil || !strings.Contains(err.Error(), "requires non-empty channel and to") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("explicit_reserved_to_rejected", func(t *testing.T) {
		m := cloneMap(base)
		m["deliver"] = true
		m["channel"] = "telegram"
		m["to"] = "deliver_to_incoming_chat"
		err := validateCronDelivery(context.Background(), m)
		if err == nil || !strings.Contains(err.Error(), "invalid to") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("none_ok", func(t *testing.T) {
		m := cloneMap(base)
		if err := validateCronDelivery(context.Background(), m); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("none_stray_to_rejected", func(t *testing.T) {
		m := cloneMap(base)
		m["to"] = "deliver_to_incoming_chat"
		err := validateCronDelivery(context.Background(), m)
		if err == nil || !strings.Contains(err.Error(), "omit channel and to") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("none_stray_channel_rejected", func(t *testing.T) {
		m := cloneMap(base)
		m["channel"] = "telegram"
		err := validateCronDelivery(context.Background(), m)
		if err == nil || !strings.Contains(err.Error(), "omit channel and to") {
			t.Fatalf("got %v", err)
		}
	})
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
