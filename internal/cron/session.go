package cron

import (
	"strings"

	"github.com/google/uuid"
)

const cronSessionKeyPrefix = "maven:cron-"

// SessionKey returns a unique agentsdk session id for one cron job run (fresh context each tick).
func SessionKey(jobID string) string {
	return cronSessionKeyPrefix + jobID + "-" + uuid.New().String()
}

// MatchesJob reports whether sessionID has the shape produced by SessionKey for jobID.
func MatchesJob(jobID, sessionID string) bool {
	p := cronSessionKeyPrefix + jobID + "-"
	if !strings.HasPrefix(sessionID, p) {
		return false
	}
	_, err := uuid.Parse(sessionID[len(p):])
	return err == nil
}
