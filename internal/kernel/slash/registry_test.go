package slash

import (
	"context"
	"testing"
)

func TestRegistry_Definitions(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(Definition{Name: "memory", Description: "Show memory"}, HandlerFunc(func(_ context.Context, _ Invocation) (Result, error) {
		return Result{}, nil
	})); err != nil {
		t.Fatalf("Register memory: %v", err)
	}
	if err := reg.Register(Definition{Name: "compact", Description: "Compact chat"}, HandlerFunc(func(_ context.Context, _ Invocation) (Result, error) {
		return Result{}, nil
	})); err != nil {
		t.Fatalf("Register compact: %v", err)
	}
	defs := reg.Definitions()
	if len(defs) != 2 {
		t.Fatalf("Definitions len = %d, want 2", len(defs))
	}
	if defs[0].Name != "compact" || defs[1].Name != "memory" {
		t.Fatalf("Definitions order = [%s %s], want [compact memory]", defs[0].Name, defs[1].Name)
	}
}
