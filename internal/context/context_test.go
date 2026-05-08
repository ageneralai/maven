package turnctx

import (
	"context"
	"testing"
)

func TestWithInbound_trimAndRetrieve(t *testing.T) {
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
	ctx := WithInbound(context.Background(), "a", "1")
	ctx = WithInbound(ctx, "b", "2")
	tc, ok := From(ctx)
	if !ok || tc.Channel != "b" || tc.ChatID != "2" {
		t.Fatalf("want overwrite, got %#v ok=%v", tc, ok)
	}
}

func TestContextManager_WithInbound(t *testing.T) {
	var m ContextManager
	ctx := m.WithInbound(context.Background(), "feishu", "c1")
	tc, ok := From(ctx)
	if !ok || tc.Channel != "feishu" {
		t.Fatalf("got %#v ok=%v", tc, ok)
	}
}
