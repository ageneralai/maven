package session

import (
	"strconv"
	"strings"
	"time"
)

const WebChannelName = "web"

// ChatSessionID is the default agentsdk session id for an inbound channel peer.
func ChatSessionID(channel, peer string) string {
	channel = strings.TrimSpace(channel)
	peer = strings.TrimSpace(peer)
	if channel == "" {
		return peer
	}
	prefix := channel + "-"
	if strings.HasPrefix(peer, prefix) {
		return peer
	}
	return prefix + peer
}

// SessionIDFromRouteKey derives the default chat session id from a StableRouteKey (channel:peer).
func SessionIDFromRouteKey(routeKey string) string {
	routeKey = strings.TrimSpace(routeKey)
	if i := strings.Index(routeKey, ":"); i > 0 && i < len(routeKey)-1 {
		return ChatSessionID(routeKey[:i], routeKey[i+1:])
	}
	return routeKey
}

// IsolatedSessionID returns a one-off session id for a single turn.
func IsolatedSessionID(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "session"
	}
	return base + "-isolated-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

// RotatedSessionID returns a new session id after /new or compact rotation.
func RotatedSessionID(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "session"
	}
	return base + "-r" + strconv.FormatInt(time.Now().UnixNano(), 10)
}
