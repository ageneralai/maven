package pipeline

import (
	"context"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/kernel/bus"
	"github.com/ageneralai/maven/kernel/channels"
	"github.com/ageneralai/maven/kernel/session"
)

type streamCapChannel struct{ channels.Channel }

func (streamCapChannel) SendStream(context.Context, string, map[string]any, <-chan api.StreamEvent) error {
	return nil
}

func TestClassifyTurn_streamWhenCapable(t *testing.T) {
	msg := bus.InboundMessage{Hints: bus.RoutingHints{SlashCommand: "compact"}}
	plan := classifyTurn(msg, streamCapChannel{})
	if !plan.useStream || plan.slashName != "compact" {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestClassifyTurn_forceSync(t *testing.T) {
	msg := bus.InboundMessage{Hints: bus.RoutingHints{ForceSync: true, SessionMode: session.SessionModeIsolated}}
	plan := classifyTurn(msg, streamCapChannel{})
	if plan.useStream {
		t.Fatal("ForceSync should disable streaming")
	}
	if plan.sessionMode != session.SessionModeIsolated {
		t.Fatalf("sessionMode=%v", plan.sessionMode)
	}
}
