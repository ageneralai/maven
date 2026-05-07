// Package cronsession assigns runtime SessionID values for scheduled cron fires.
// Each execution gets a fresh key so LLM/tool context does not accumulate across ticks.
package cronsession

import (
	"strings"

	"github.com/google/uuid"
)

const keyPrefix = "maven:cron-"

// SessionKey returns a unique agentsdk session id for one run of jobID.
func SessionKey(jobID string) string {
	return keyPrefix + jobID + "-" + uuid.New().String()
}

// MatchesJob reports whether sessionID has the shape produced by SessionKey for jobID.
func MatchesJob(jobID, sessionID string) bool {
	p := keyPrefix + jobID + "-"
	if !strings.HasPrefix(sessionID, p) {
		return false
	}
	_, err := uuid.Parse(sessionID[len(p):])
	return err == nil
}
