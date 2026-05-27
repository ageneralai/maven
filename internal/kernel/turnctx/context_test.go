package turnctx

import (
	"context"
	"testing"
)

func TestWithInbound_trimAndRetrieve(t *testing.T) {
	t.Parallel()
	ctx := WithInbound(context.Background(), "  telegram \n", " 42 ")
	tc, ok := From(ctx)
	if !ok || tc.Channel != "telegram" || tc.ChatID != "42" {
		t.Fatalf("From = %#v ok=%v", tc, ok)
	}
	ch, okCh := Channel(ctx)
	id, okID := ChatID(ctx)
	if !okCh || ch != "telegram" || !okID || id != "42" {
		t.Fatalf("Channel=%q %v ChatID=%q %v", ch, okCh, id, okID)
	}
}

func TestFrom_missingIncomplete(t *testing.T) {
	t.Parallel()
	if _, ok := From(context.Background()); ok {
		t.Fatal("want no snapshot")
	}
	if _, ok := From(WithInbound(context.Background(), "", "x")); ok {
		t.Fatal("want missing channel rejects")
	}
	if _, ok := From(WithInbound(context.Background(), "c", "")); ok {
		t.Fatal("want missing chat id rejects")
	}
}

func TestWith_overwrites(t *testing.T) {
	t.Parallel()
	ctx := WithInbound(context.Background(), "a", "1")
	ctx = WithInbound(ctx, "b", "2")
	tc, ok := From(ctx)
	if !ok || tc.Channel != "b" || tc.ChatID != "2" {
		t.Fatalf("want overwrite, got %#v ok=%v", tc, ok)
	}
}

func TestWithInbound_preservesMetadata(t *testing.T) {
	t.Parallel()
	ctx := WithInbound(context.Background(), "a", "1")
	ctx = WithMetadata(ctx, map[string]any{"  ping ": true})
	ctx = WithInbound(ctx, "b", " 2 ")
	tc, ok := From(ctx)
	if !ok {
		t.Fatalf("From missing")
	}
	if tc.Channel != "b" || tc.ChatID != "2" {
		t.Fatalf("channel/chat: %#v", tc)
	}
	if tc.Metadata == nil || tc.Metadata["ping"] != true {
		t.Fatalf("metadata: %#v", tc.Metadata)
	}
}

func TestWith_explicitMetadataClearsPreserve(t *testing.T) {
	t.Parallel()
	ctx := WithInbound(context.Background(), "a", "1")
	ctx = WithMetadata(ctx, map[string]any{"k": 1})
	ctx = With(ctx, TurnContext{Channel: "a", ChatID: "1", Metadata: map[string]any{"only": struct{}{}}})
	tc, ok := From(ctx)
	if !ok || len(tc.Metadata) != 1 || tc.Metadata["only"] == nil {
		t.Fatalf("got %#v", tc.Metadata)
	}
}

func TestWith_nilMetadataPreserves(t *testing.T) {
	t.Parallel()
	ctx := WithInbound(context.Background(), "x", "y")
	ctx = WithMetadata(ctx, map[string]any{"k": 1})
	ctx = With(ctx, TurnContext{Channel: "x", ChatID: "y"})
	tc, ok := From(ctx)
	if !ok || len(tc.Metadata) != 1 || tc.Metadata["k"] != 1 {
		t.Fatalf("metadata %#v", tc.Metadata)
	}
}

func TestNormalize_metadataKeyTrimSkipsWhitespaceOnlyKeys(t *testing.T) {
	t.Parallel()
	ctx := WithInbound(context.Background(), "c", "d")
	ctx = WithMetadata(ctx, map[string]any{"  hello ": "v", "\t ": "ignored"})
	tc, ok := From(ctx)
	if !ok || len(tc.Metadata) != 1 || tc.Metadata["hello"] != "v" {
		t.Fatalf("got %#v", tc.Metadata)
	}
}
