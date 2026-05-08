package turnctx

import (
	"context"
	"strings"
)

// TurnContext carries immutable routing identity for one gateway inbound turn (pipeline
// handle). Downstream retrieves it via From, Channel, or ChatID instead of bespoke keys.
type TurnContext struct {
	Channel string
	ChatID  string
}

type turnKey struct{}

// ContextManager attaches turn identity to contexts. The zero value is valid and
// stateless; callers can replace it later if per-gateway policy needs options.
type ContextManager struct{}

// WithInbound is equivalent to package-level WithInbound; use as a façade when wiring
// a manager field without extra options.
func (ContextManager) WithInbound(parent context.Context, channel, chatID string) context.Context {
	return WithInbound(parent, channel, chatID)
}

func normalize(tc TurnContext) TurnContext {
	return TurnContext{Channel: strings.TrimSpace(tc.Channel), ChatID: strings.TrimSpace(tc.ChatID)}
}

// With attaches tc to parent. Duplicate keys replace the previous turn snapshot for this key.
func With(parent context.Context, tc TurnContext) context.Context {
	return context.WithValue(parent, turnKey{}, normalize(tc))
}

// WithInbound is pipeline shorthand for With(TurnContext{channel, chatID}); spaces trimmed.
func WithInbound(parent context.Context, channel, chatID string) context.Context {
	return With(parent, TurnContext{Channel: channel, ChatID: chatID})
}

// From returns the attached turn plus ok when both Channel and ChatID are non-empty.
func From(ctx context.Context) (TurnContext, bool) {
	tc, ok := ctx.Value(turnKey{}).(TurnContext)
	if !ok {
		return TurnContext{}, false
	}
	tc = normalize(tc)
	if tc.Channel == "" || tc.ChatID == "" {
		return TurnContext{}, false
	}
	return tc, true
}

// Channel returns the trimmed channel name and reports whether a full turn snapshot is present.
func Channel(ctx context.Context) (string, bool) {
	tc, ok := From(ctx)
	if !ok {
		return "", false
	}
	return tc.Channel, true
}

// ChatID returns the trimmed chat id when a full turn snapshot is present (same readiness as Channel).
func ChatID(ctx context.Context) (string, bool) {
	tc, ok := From(ctx)
	if !ok {
		return "", false
	}
	return tc.ChatID, true
}
