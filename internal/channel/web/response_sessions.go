package web

import (
	"strings"
	"sync"
	"time"
)

const (
	HeaderMavenSessionID     = "Maven-Session-Id"
	responseSessionTTL       = 30 * time.Minute
	maxResponseSessionEntries = 500
)

type responseSessionEntry struct {
	sessionID string
	ts        time.Time
}

var (
	responseSessionMu sync.Mutex
	responseSessions  = map[string]responseSessionEntry{}
)

func isMavenResponseID(id string) bool {
	id = strings.TrimSpace(id)
	return strings.HasPrefix(id, "resp_") && len(id) > len("resp_")
}

func storeMavenResponseSession(responseID, sessionID string) {
	responseID = strings.TrimSpace(responseID)
	sessionID = strings.TrimSpace(sessionID)
	if responseID == "" || sessionID == "" {
		return
	}
	now := time.Now()
	responseSessionMu.Lock()
	defer responseSessionMu.Unlock()
	pruneMavenResponseSessionsLocked(now)
	delete(responseSessions, responseID)
	responseSessions[responseID] = responseSessionEntry{sessionID: sessionID, ts: now}
	evictMavenResponseSessionsLocked()
}

func lookupMavenResponseSession(responseID string) (string, bool) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return "", false
	}
	now := time.Now()
	responseSessionMu.Lock()
	defer responseSessionMu.Unlock()
	entry, ok := responseSessions[responseID]
	if !ok {
		return "", false
	}
	if now.Sub(entry.ts) > responseSessionTTL {
		delete(responseSessions, responseID)
		return "", false
	}
	delete(responseSessions, responseID)
	responseSessions[responseID] = responseSessionEntry{sessionID: entry.sessionID, ts: now}
	return entry.sessionID, true
}

func pruneMavenResponseSessionsLocked(now time.Time) {
	for id, entry := range responseSessions {
		if now.Sub(entry.ts) > responseSessionTTL {
			delete(responseSessions, id)
		}
	}
}

func evictMavenResponseSessionsLocked() {
	for len(responseSessions) > maxResponseSessionEntries {
		var oldestID string
		var oldest time.Time
		for id, entry := range responseSessions {
			if oldestID == "" || entry.ts.Before(oldest) {
				oldestID = id
				oldest = entry.ts
			}
		}
		if oldestID == "" {
			return
		}
		delete(responseSessions, oldestID)
	}
}

func resetMavenResponseSessionsForTest() {
	responseSessionMu.Lock()
	responseSessions = map[string]responseSessionEntry{}
	responseSessionMu.Unlock()
}
