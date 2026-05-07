// Package heartbeatsession assigns agentsdk SessionID values for periodic heartbeat ticks.
//
// agentsdk-go keys conversation history only by SessionID; there is no Request-level skip-history flag.
// A fresh key each tick matches PicoClaw heartbeat NoHistory semantics (no stacked prior heartbeat context).
package heartbeatsession

import (
	"strings"

	"github.com/google/uuid"
)

const keyPrefix = "maven:heartbeat-"

// SessionKey returns a unique session id for one heartbeat run.
func SessionKey() string {
	return keyPrefix + uuid.New().String()
}

// Matches reports whether sessionID was produced by SessionKey.
func Matches(sessionID string) bool {
	if !strings.HasPrefix(sessionID, keyPrefix) {
		return false
	}
	_, err := uuid.Parse(strings.TrimPrefix(sessionID, keyPrefix))
	return err == nil
}
