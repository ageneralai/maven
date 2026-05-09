package slash

import (
	"context"
	"maps"

	turnctx "github.com/ageneralai/maven/pkg/context"
)

// enrichRequestMetadataWithTurnRouting merges handler metadata with channel/chat from ctx when
// turnctx.From succeeds (pipeline must pass the per-turn msgCtx).
func enrichRequestMetadataWithTurnRouting(ctx context.Context, md map[string]any) map[string]any {
	out := maps.Clone(md)
	if out == nil {
		out = make(map[string]any)
	}
	if tc, ok := turnctx.From(ctx); ok {
		if _, exists := out["slash.turn.channel"]; !exists {
			out["slash.turn.channel"] = tc.Channel
		}
		if _, exists := out["slash.turn.chat_id"]; !exists {
			out["slash.turn.chat_id"] = tc.ChatID
		}
	}
	return out
}
