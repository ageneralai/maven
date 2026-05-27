package sessionid

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Kind string

const (
	KindChat      Kind = "chat"
	KindCron      Kind = "cron"
	KindHeartbeat Kind = "heartbeat"
	KindIsolated  Kind = "isolated"
	KindRotated   Kind = "rotated"
	KindTask      Kind = "task"
)

// Prefix returns the string prefix for a given kind (for HasPrefix checks).
func Prefix(kind Kind) string {
	switch kind {
	case KindCron:
		return "cron:"
	case KindHeartbeat:
		return "heartbeat:"
	case KindTask:
		return "task:"
	default:
		return ""
	}
}

// New returns a session ID for the given kind and seed.
func New(kind Kind, seed string) string {
	seed = strings.TrimSpace(seed)
	switch kind {
	case KindCron:
		return fmt.Sprintf("cron:%s:%s", seed, uuid.NewString())
	case KindHeartbeat:
		return "heartbeat:" + uuid.NewString()
	case KindIsolated:
		if seed == "" {
			seed = "session"
		}
		return fmt.Sprintf("%s:isolated:%d", seed, time.Now().UnixNano())
	case KindRotated:
		if seed == "" {
			seed = "session"
		}
		return fmt.Sprintf("%s:rotated:%d", seed, time.Now().UnixNano())
	case KindTask:
		return "task:" + uuid.NewString()
	default:
		return seed
	}
}

// Match parses a session ID and returns its kind, seed, and whether it was produced by New.
func Match(s string) (Kind, string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", false
	}
	if strings.HasPrefix(s, "cron:") {
		rest := strings.TrimPrefix(s, "cron:")
		jobID, id, ok := strings.Cut(rest, ":")
		if !ok || jobID == "" {
			return "", "", false
		}
		if _, err := uuid.Parse(id); err != nil {
			return "", "", false
		}
		return KindCron, jobID, true
	}
	if strings.HasPrefix(s, "heartbeat:") {
		id := strings.TrimPrefix(s, "heartbeat:")
		if _, err := uuid.Parse(id); err != nil {
			return "", "", false
		}
		return KindHeartbeat, "", true
	}
	if strings.HasPrefix(s, "task:") {
		id := strings.TrimPrefix(s, "task:")
		if _, err := uuid.Parse(id); err != nil {
			return "", "", false
		}
		return KindTask, "", true
	}
	if base, suffix, ok := strings.Cut(s, ":isolated:"); ok {
		if _, err := strconv.ParseInt(suffix, 10, 64); err != nil {
			return "", "", false
		}
		return KindIsolated, base, true
	}
	if base, suffix, ok := strings.Cut(s, ":rotated:"); ok {
		if _, err := strconv.ParseInt(suffix, 10, 64); err != nil {
			return "", "", false
		}
		return KindRotated, base, true
	}
	return KindChat, s, true
}

// MatchesCronJob reports whether sessionID was produced by New(KindCron, jobID).
func MatchesCronJob(jobID, sessionID string) bool {
	kind, seed, ok := Match(sessionID)
	return ok && kind == KindCron && seed == strings.TrimSpace(jobID)
}

// MatchesHeartbeat reports whether sessionID was produced by New(KindHeartbeat, "").
func MatchesHeartbeat(sessionID string) bool {
	kind, _, ok := Match(sessionID)
	return ok && kind == KindHeartbeat
}

// MatchesTask reports whether sessionID was produced by New(KindTask, "").
func MatchesTask(sessionID string) bool {
	kind, _, ok := Match(sessionID)
	return ok && kind == KindTask
}
