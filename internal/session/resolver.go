package session

import (
	"github.com/ageneralai/maven/internal/bus"
)

// Resolver maps inbound routing identity to agentsdk SessionID strings.
type Resolver interface {
	ResolveSDKSessionID(msg bus.InboundMessage) string
}

// SessionResolver maps inbound routing identity to agentsdk SessionID strings.
type SessionResolver struct {
	Router *Router
}

func (r *SessionResolver) ResolveSDKSessionID(msg bus.InboundMessage) string {
	routeKey := msg.StableRouteKey()
	base := ChatSessionID(msg.Channel, msg.ChatID)
	if msg.Hints.SessionMode == bus.SessionModeIsolated {
		return IsolatedSessionID(base)
	}
	if r == nil || r.Router == nil {
		return base
	}
	return r.Router.Resolve(routeKey, base)
}

var _ Resolver = (*SessionResolver)(nil)
