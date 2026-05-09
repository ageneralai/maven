package slash

import (
	"context"
	"reflect"
	"testing"

	turnctx "github.com/ageneralai/maven/pkg/context"
)

func TestPreTurn_EmptyOutput_ContinuesToModelWithTrail(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(
		Definition{Name: "blank", Description: ""},
		HandlerFunc(func(_ context.Context, _ Invocation) (Result, error) {
			return Result{Output: "", Metadata: map[string]any{"k": true}}, nil
		}),
	); err != nil {
		t.Fatal(err)
	}
	ctx := turnctx.WithInbound(context.Background(), "telegram", "1")
	got, err := PreTurn(ctx, reg, Input{Text: "/blank"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.ContinueToModel {
		t.Fatalf("ContinueToModel got false")
	}
	if got.DirectReply != "" {
		t.Fatalf("DirectReply %q", got.DirectReply)
	}
	if len(got.RequestMetadata) == 0 {
		t.Fatal("want RequestMetadata from handler")
	}
	if got.RequestMetadata["k"] != true {
		t.Fatalf("metadata %v", got.RequestMetadata)
	}
	if len(got.Trail) != 1 || got.Trail[0].Result.Output != "" {
		t.Fatalf("trail %#v", got.Trail)
	}
}

func TestPreTurn_NonEmptyOutput_BypassesModel(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(
		Definition{Name: "hi", Description: ""},
		HandlerFunc(func(_ context.Context, _ Invocation) (Result, error) {
			return Result{Output: "  hello body  ", Metadata: map[string]any{"omit": false}}, nil
		}),
	); err != nil {
		t.Fatal(err)
	}
	got, err := PreTurn(context.Background(), reg, Input{Text: "/hi"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ContinueToModel {
		t.Fatal("want model skipped")
	}
	if got.DirectReply != "hello body" {
		t.Fatalf("DirectReply got %q", got.DirectReply)
	}
}

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
