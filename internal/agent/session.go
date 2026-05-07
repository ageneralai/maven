package agent

import (
	"strconv"
	"time"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/session"
)

// SessionResolver maps inbound routing identity to agentsdk SessionID strings.
type SessionResolver struct {
	Router *session.Router
}

func (r *SessionResolver) ResolveSDKSessionID(msg bus.InboundMessage) string {
	base := msg.StableRouteKey()
	if msg.Hints.SessionMode == bus.SessionModeIsolated {
		return base + "#isolated#" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	if r == nil || r.Router == nil {
		return base
	}
	return r.Router.Resolve(base, base)
}
