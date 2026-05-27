package session

import (
	"github.com/ageneralai/maven/internal/sessionid"
)

// Resolver maps inbound routing identity to agentsdk SessionID strings.
type Resolver interface {
	ResolveSDKSessionID(channel, chatID, routeKey string, mode SessionMode) string
}

// SessionResolver maps inbound routing identity to agentsdk SessionID strings.
type SessionResolver struct {
	Router *Router
}

func (r *SessionResolver) ResolveSDKSessionID(channel, chatID, routeKey string, mode SessionMode) string {
	base := sessionid.ChatSessionID(channel, chatID)
	if mode == SessionModeIsolated {
		return sessionid.New(sessionid.KindIsolated, base).String()
	}
	if r == nil || r.Router == nil {
		return base
	}
	return r.Router.Resolve(routeKey, base)
}

var _ Resolver = (*SessionResolver)(nil)
