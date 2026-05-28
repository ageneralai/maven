package cron

import (
	"context"
	"testing"

	"log/slog"
)

func TestListTool_returnsServiceNotReadyWhenNilSvc(t *testing.T) {
	t.Parallel()
	p := &Plugin{log: toolTestLog}
	tools := Tools(p, slog.New(slog.DiscardHandler))
	if len(tools) != 3 {
		t.Fatalf("tools=%d", len(tools))
	}
	ctx := context.Background()
	for i, wantName := range []string{"cron-schedule", "cron-list", "cron-remove"} {
		res, err := tools[i].Execute(ctx, nil)
		if err != nil {
			t.Fatalf("%s: %v", wantName, err)
		}
		if res.Success {
			t.Fatalf("%s: Success=true, want false", wantName)
		}
		if res.Output != "cron service not ready" {
			t.Fatalf("%s: Output=%q, want cron service not ready", wantName, res.Output)
		}
		if tools[i].Name() != wantName {
			t.Fatalf("tools[%d].Name()=%q, want %q", i, tools[i].Name(), wantName)
		}
	}
}
