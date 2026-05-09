package slash

import (
	"context"
	"reflect"
	"testing"

	turnctx "github.com/ageneralai/maven/internal/context"
)

func TestEnrichRequestMetadataWithTurnRouting(t *testing.T) {
	baseCtx := turnctx.WithInbound(context.Background(), "telegram", "4242")
	tests := []struct {
		name string
		ctx  context.Context
		md   map[string]any
		want map[string]any
	}{
		{
			name: "injects_routing_when_absent",
			ctx:  baseCtx,
			md:   map[string]any{"k": float64(1)},
			want: map[string]any{
				"k":                    float64(1),
				"slash.turn.channel":   "telegram",
				"slash.turn.chat_id":   "4242",
			},
		},
		{
			name: "does_not_overwrite_existing_slash_turn_channel",
			ctx:  baseCtx,
			md: map[string]any{
				"slash.turn.channel": "keep-from-handler",
				"k":                  true,
			},
			want: map[string]any{
				"slash.turn.channel": "keep-from-handler",
				"k":                  true,
				"slash.turn.chat_id": "4242",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enrichRequestMetadataWithTurnRouting(tt.ctx, tt.md)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v want %#v", got, tt.want)
			}
		})
	}
}
