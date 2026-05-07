// Package inboundctx holds typed context keys for the gateway inbound channel/chat.
// Use these instead of string keys so unrelated packages cannot collide on ctx.Value.
package inboundctx

import (
	"context"
	"strings"
)

type key int

const (
	kChannel key = iota + 1
	kChatID
)

// With returns ctx with inbound channel and chat id stored for tool handlers (e.g. CronSchedule deliver_to_incoming_chat).
func With(ctx context.Context, channel, chatID string) context.Context {
	ctx = context.WithValue(ctx, kChannel, channel)
	return context.WithValue(ctx, kChatID, chatID)
}

// Channel returns the trimmed inbound channel name and whether it is non-empty.
func Channel(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(kChannel).(string)
	s = strings.TrimSpace(s)
	return s, ok && s != ""
}

// ChatID returns the trimmed inbound chat id and whether it is non-empty.
func ChatID(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(kChatID).(string)
	s = strings.TrimSpace(s)
	return s, ok && s != ""
}
