package heartbeat

import (
	"strings"

	"github.com/google/uuid"
)

const heartbeatSessionKeyPrefix = "maven:heartbeat-"

// SessionKey returns a unique session id for one heartbeat run (no stacked prior heartbeat context).
func SessionKey() string {
	return heartbeatSessionKeyPrefix + uuid.New().String()
}

// MatchesSession reports whether sessionID was produced by SessionKey.
func MatchesSession(sessionID string) bool {
	if !strings.HasPrefix(sessionID, heartbeatSessionKeyPrefix) {
		return false
	}
	_, err := uuid.Parse(strings.TrimPrefix(sessionID, heartbeatSessionKeyPrefix))
	return err == nil
}
