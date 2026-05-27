package session

import (
	"strings"
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
