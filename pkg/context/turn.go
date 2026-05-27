package turnctx

import (
	"context"
	"strings"
)

// TurnContext carries immutable routing identity for one gateway inbound turn (pipeline
// handle). Downstream retrieves it via From, Channel, or ChatID instead of bespoke keys.
type TurnContext struct {
	Channel  string
	ChatID   string
	Metadata map[string]any
}

type turnKey struct{}

func load(parent context.Context) (TurnContext, bool) {
	tc, ok := parent.Value(turnKey{}).(TurnContext)
	return tc, ok
}

func mergePreserve(prev, next TurnContext) TurnContext {
	out := TurnContext{Channel: next.Channel, ChatID: next.ChatID}
	if next.Metadata != nil {
		out.Metadata = next.Metadata
	} else {
		out.Metadata = prev.Metadata
	}
	return out
}

func normalize(tc TurnContext) TurnContext {
	out := TurnContext{
		Channel: strings.TrimSpace(tc.Channel),
		ChatID:  strings.TrimSpace(tc.ChatID),
	}
	if tc.Metadata != nil {
		meta := make(map[string]any, len(tc.Metadata))
		for k, v := range tc.Metadata {
			nk := strings.TrimSpace(k)
			if nk == "" {
				continue
			}
			meta[nk] = v
		}
		if len(meta) > 0 {
			out.Metadata = meta
		}
	}
	return out
}

func overlayMetadata(meta map[string]any, overlay map[string]any) map[string]any {
	var base map[string]any
	if len(meta) > 0 {
		base = make(map[string]any, len(meta)+len(overlay))
		for k, v := range meta {
			base[k] = v
		}
	} else if len(overlay) > 0 {
		base = make(map[string]any, len(overlay))
	}
	if base == nil {
		return nil
	}
	for k, v := range overlay {
		nk := strings.TrimSpace(k)
		if nk == "" {
			continue
		}
		base[nk] = v
	}
	return base
}

// With attaches tc to parent. When tc.Metadata is nil, Metadata from an existing
// snapshot on parent is retained; Channel and ChatID always come from tc (after normalization).
func With(parent context.Context, tc TurnContext) context.Context {
	prev, ok := load(parent)
	if !ok {
		prev = TurnContext{}
	}
	merged := mergePreserve(prev, tc)
	return context.WithValue(parent, turnKey{}, normalize(merged))
}

// WithInbound is pipeline shorthand that updates channel/chat id while preserving Metadata.
func WithInbound(parent context.Context, channel, chatID string) context.Context {
	return With(parent, TurnContext{Channel: channel, ChatID: chatID})
}

// WithMetadata overlays metadata keys (trimmed) onto the snapshot; nil overlay is a no-op.
func WithMetadata(parent context.Context, metadata map[string]any) context.Context {
	if metadata == nil {
		return parent
	}
	prev, ok := load(parent)
	if !ok {
		prev = TurnContext{}
	}
	prev.Metadata = overlayMetadata(prev.Metadata, metadata)
	return context.WithValue(parent, turnKey{}, normalize(prev))
}

// From returns the attached turn plus ok when both Channel and ChatID are non-empty.
// Metadata is optional and included when ok is true.
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

// IntFromAny converts typical numeric dynamic values to int (metadata map entries, JSON numbers).
// int, int32, int64, float64 map to int; everything else yields 0.
func IntFromAny(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}
